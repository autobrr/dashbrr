/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Service,
  ServiceStatus,
  ServiceType,
  ServiceStats,
  ServiceDetails,
  AutobrrStats,
  AutobrrIRC,
  MaintainerrCollection,
  PlexSession,
  OverseerrStats,
  SonarrQueue,
  RadarrQueue,
  ProwlarrStats,
  ProwlarrIndexer,
  ServiceConfig
} from '../types/service';
import { useConfiguration } from '../contexts/useConfiguration';
import { useAuth } from '../contexts/AuthContext';
import serviceTemplates from '../config/serviceTemplates';
import { api } from '../utils/api';
import { cache, CACHE_PREFIXES } from '../utils/cache';

interface ServiceHealth {
  status: ServiceStatus;
  responseTime: number;
  message?: string;
  version?: string;
  updateAvailable?: boolean;
}

interface ServiceData {
  stats?: ServiceStats;
  details?: ServiceDetails;
}

// Background polling intervals
const STATS_CHECK_INTERVAL = 300000;   // 5 minutes for service stats
const PLEX_SESSIONS_INTERVAL = 30000;  // 30 seconds for Plex sessions
const INITIAL_FETCH_DELAY = 100;       // 100ms initial fetch delay

function debounce<T extends (...args: Parameters<T>) => ReturnType<T>>(
  fn: T,
  ms: number
): T {
  let timeoutId: NodeJS.Timeout;
  return function(this: unknown, ...args: Parameters<T>) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn.apply(this, args), ms);
  } as T;
}

export const useServiceData = () => {
  const { configurations } = useConfiguration();
  const { isAuthenticated } = useAuth();
  const [services, setServices] = useState<Map<string, Service>>(new Map());
  const [isLoading, setIsLoading] = useState(true);
  const updateTimeoutsRef = useRef<Map<string, NodeJS.Timeout>>(new Map());
  const plexTimeoutRef = useRef<NodeJS.Timeout>();
  const configHashRef = useRef<string>('');

  const clearServiceTimeout = useCallback((serviceId: string) => {
    const timeoutId = updateTimeoutsRef.current.get(serviceId);
    if (timeoutId) {
      clearTimeout(timeoutId);
      updateTimeoutsRef.current.delete(serviceId);
    }
  }, []);

  const updateServiceData = useCallback((serviceId: string, data: Partial<Service>) => {
    setServices(prev => {
      const newServices = new Map(prev);
      const currentService = newServices.get(serviceId);
      if (currentService) {
        // Deep merge stats and details
        const mergedService = {
          ...currentService,
          ...data,
          lastChecked: new Date(),
          stats: data.stats ? {
            ...currentService.stats,
            ...data.stats
          } : currentService.stats,
          details: data.details ? {
            ...currentService.details,
            ...data.details
          } : currentService.details
        };
        newServices.set(serviceId, mergedService);
      }
      return newServices;
    });
  }, []);

  const fetchPlexSessions = useCallback(async (service: Service) => {
    try {
      const response = await api.get<{ MediaContainer?: { Metadata?: PlexSession[] } }>(
        `/api/plex/sessions?instanceId=${service.instanceId}`
      );
      const sessions = response?.MediaContainer?.Metadata || [];
      const data = {
        stats: { plex: { sessions } },
        details: {
          plex: {
            activeStreams: sessions.length,
            transcoding: sessions.filter(s => s?.TranscodeSession).length
          }
        }
      };
      updateServiceData(service.instanceId, data);
    } catch (error) {
      console.error('Error fetching Plex sessions:', error);
    }
  }, [updateServiceData]);

  const fetchServiceStats = useCallback(async (service: Service) => {
    if (service.type === 'omegabrr' || service.type === 'tailscale' || service.type === 'general') return;
    if (!service.url || !service.apiKey) return;

    // Special handling for Plex sessions
    if (service.type === 'plex') {
      await fetchPlexSessions(service);
      return;
    }

    const statsCacheKey = `${CACHE_PREFIXES.STATS}${service.instanceId}`;
    const { data: cachedStats, isStale } = cache.get<ServiceData>(statsCacheKey);

    // Use cached data if available and not stale
    if (cachedStats && !isStale) {
      updateServiceData(service.instanceId, cachedStats);
      return;
    }

    let data: ServiceData | undefined;

    try {
      switch (service.type) {
        case 'overseerr': {
          const stats = await api.get<OverseerrStats>(
            `/api/overseerr/requests?instanceId=${service.instanceId}`
          );
          data = { stats: { overseerr: stats }, details: {} };
          break;
        }
        case 'sonarr': {
          const queueData = await api.get<SonarrQueue>(`/api/sonarr/queue?instanceId=${service.instanceId}`);
          if (queueData) {
            data = {
              stats: { sonarr: { queue: queueData } },
            };
          }
          break;
        }
        case 'radarr': {
          const queueData = await api.get<RadarrQueue>(`/api/radarr/queue?instanceId=${service.instanceId}`);
          if (queueData) {
            data = {
              stats: { radarr: { queue: queueData } },
            };
          }
          break;
        }
        case 'prowlarr': {
          const [statsData, indexersData] = await Promise.all([
            api.get<ProwlarrStats>(`/api/prowlarr/stats?instanceId=${service.instanceId}`),
            api.get<ProwlarrIndexer[]>(`/api/prowlarr/indexers?instanceId=${service.instanceId}`)
          ]);
          if (statsData && indexersData) {
            data = {
              stats: { 
                prowlarr: { 
                  stats: statsData, 
                  indexers: indexersData,
                  prowlarrIndexerStats: {
                    id: 1,
                    indexers: []
                  }
                } 
              },
              details: { 
                prowlarr: { 
                  activeIndexers: indexersData.filter(i => i.enable).length, 
                  totalGrabs: statsData.grabCount 
                } 
              }
            };
          }
          break;
        }
        case 'autobrr': {
          const [statsData, ircData] = await Promise.all([
            api.get<AutobrrStats>(`/api/autobrr/stats?instanceId=${service.instanceId}`),
            api.get<AutobrrIRC[]>(`/api/autobrr/irc?instanceId=${service.instanceId}`)
          ]);
          if (statsData && ircData) {
            data = { stats: { autobrr: statsData }, details: { autobrr: { irc: ircData } } };
          }
          break;
        }
        case 'maintainerr': {
          const collections = await api.get<MaintainerrCollection[]>(
            `/api/maintainerr/collections?instanceId=${service.instanceId}`
          );
          data = {
            stats: { maintainerr: { collections } },
            details: {
              maintainerr: {
                activeCollections: collections.filter(c => c.isActive).length,
                totalMedia: collections.reduce((acc, c) => acc + c.media.length, 0)
              }
            }
          };
          break;
        }
      }

      if (data) {
        cache.set(statsCacheKey, data);
        updateServiceData(service.instanceId, data);
      }
    } catch (error) {
      console.error(`Error fetching stats for ${service.type}:`, error);
    }
  }, [updateServiceData, fetchPlexSessions]);

  const fetchHealthStatus = useCallback(async (service: Service) => {
    const healthCacheKey = `${CACHE_PREFIXES.HEALTH}${service.instanceId}`;
    const { data: cachedHealth, isStale } = cache.get<ServiceHealth>(healthCacheKey);

    // Use cached data if available and not stale
    if (cachedHealth && !isStale) {
      updateServiceData(service.instanceId, cachedHealth);
      return;
    }

    try {
      const health = await api.get<ServiceHealth>(`/api/health/${service.instanceId}`);
      if (health) {
        cache.set(healthCacheKey, health);
        updateServiceData(service.instanceId, health);
      }
    } catch (error) {
      console.error(`Error fetching health for ${service.type}:`, error);
    }
  }, [updateServiceData]);

  const initializeService = useCallback((instanceId: string, config: ServiceConfig) => {
    const [type] = instanceId.split('-');
    const template = serviceTemplates.find(t => t.type === type);
    const hasRequiredConfig = Boolean(config.url && (config.apiKey || type === 'general'));

    const service = {
      id: instanceId,
      instanceId,
      name: template?.name || 'Unknown Service',
      type: (template?.type || 'other') as ServiceType,
      status: hasRequiredConfig ? 'loading' as ServiceStatus : 'pending' as ServiceStatus,
      url: config.url,
      accessUrl: config.accessUrl,
      apiKey: config.apiKey,
      displayName: config.displayName,
      healthEndpoint: template?.healthEndpoint,
      message: hasRequiredConfig ? 'Loading service status' : 'Service not configured',
      stats: {},
      details: {}
    } as Service;

    setServices(prev => {
      const newServices = new Map(prev);
      newServices.set(instanceId, service);
      return newServices;
    });

    return service;
  }, []);

  const updateService = useCallback((service: Service) => {
    clearServiceTimeout(service.instanceId);

    // Initial fetch with minimal delay
    setTimeout(() => {
      Promise.all([
        fetchHealthStatus(service),
        fetchServiceStats(service)
      ]).catch(console.error);
    }, INITIAL_FETCH_DELAY);

    // Schedule background polling
    const timeoutId = setTimeout(() => {
      updateService(service);
    }, service.type === 'plex' ? PLEX_SESSIONS_INTERVAL : STATS_CHECK_INTERVAL);

    updateTimeoutsRef.current.set(service.instanceId, timeoutId);
  }, [clearServiceTimeout, fetchHealthStatus, fetchServiceStats]);

  // Handle configuration changes
  useEffect(() => {
    if (!isAuthenticated || !configurations) {
      setServices(new Map());
      setIsLoading(false);
      return;
    }

    // Generate hash of current configurations
    const configHash = JSON.stringify(configurations);
    if (configHash === configHashRef.current) {
      return; // No changes in configurations
    }
    configHashRef.current = configHash;

    // Handle service additions and updates
    Object.entries(configurations).forEach(([instanceId, config]) => {
      const existingService = services.get(instanceId);
      const configChanged = existingService && (
        existingService.url !== config.url ||
        existingService.apiKey !== config.apiKey ||
        existingService.displayName !== config.displayName
      );

      if (!existingService || configChanged) {
        const service = initializeService(instanceId, config);
        updateService(service);
      }
    });

    // Handle service removals
    const currentServiceIds = new Set(Object.keys(configurations));
    const existingServiceIds = Array.from(services.keys());
    const removedServiceIds = existingServiceIds.filter(id => !currentServiceIds.has(id));

    if (removedServiceIds.length > 0) {
      setServices(prev => {
        const newServices = new Map(prev);
        removedServiceIds.forEach(id => {
          clearServiceTimeout(id);
          newServices.delete(id);
        });
        return newServices;
      });
    }

    setIsLoading(false);

    return () => {
      Array.from(services.keys()).forEach(clearServiceTimeout);
      if (plexTimeoutRef.current) {
        clearTimeout(plexTimeoutRef.current);
      }
    };
  }, [configurations, isAuthenticated, clearServiceTimeout, initializeService, updateService]);

  const refreshService = useCallback((instanceId: string, refreshType: 'health' | 'stats' | 'all' = 'all'): void => {
    const service = services.get(instanceId);
    if (!service) return;
    
    // Clear only the specified cache types
    if (refreshType === 'all' || refreshType === 'health') {
      cache.remove(`${CACHE_PREFIXES.HEALTH}${instanceId}`);
    }
    if (refreshType === 'all' || refreshType === 'stats') {
      cache.remove(`${CACHE_PREFIXES.STATS}${instanceId}`);
    }
    
    // Force update
    updateService(service);
  }, [services, updateService]);

  const debouncedRefreshService = useCallback((
    instanceId: string,
    refreshType: 'health' | 'stats' | 'all' = 'all'
  ) => {
    debounce((id: string, type: typeof refreshType) => {
      refreshService(id, type);
    }, 1000)(instanceId, refreshType);
  }, [refreshService]);

  return {
    services: Array.from(services.values()),
    isLoading,
    refreshService: debouncedRefreshService
  };
};
