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
  ProwlarrIndexer
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
const HEALTH_CHECK_INTERVAL = 300000;  // 5 minutes for health checks
const STATS_CHECK_INTERVAL = 300000;   // 5 minutes for service stats
const PLEX_SESSIONS_INTERVAL = 30000;  // 30 seconds for Plex sessions

// Service-specific intervals
const SERVICE_STATS_INTERVALS: Record<ServiceType, number> = {
  plex: PLEX_SESSIONS_INTERVAL,
  autobrr: 300000,   // 5 minutes for autobrr
  overseerr: 300000, // 5 minutes for overseerr
  sonarr: 300000,    // 5 minutes for sonarr
  maintainerr: 300000,// 5 minutes for maintainerr
  omegabrr: 600000,  // 10 minutes (health check only)
  radarr: 300000,    // 5 minutes for radarr
  prowlarr: 300000,  // 5 minutes for prowlarr
  tailscale: 600000, // 10 minutes (health check only)
  other: 300000      // 5 minutes default
};

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

  const clearServiceTimeouts = useCallback((serviceId: string) => {
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
        newServices.set(serviceId, {
          ...currentService,
          ...data,
          lastChecked: new Date()
        } as Service);
      }
      return newServices;
    });
  }, []);

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
    if (service.type === 'omegabrr' || service.type === 'tailscale') return;
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
              stats: { prowlarr: { stats: statsData, indexers: indexersData } },
              details: { prowlarr: { activeIndexers: indexersData.filter(i => i.enable).length, totalGrabs: statsData.grabCount } }
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

  const updateService = useCallback((service: Service) => {
    clearServiceTimeouts(service.instanceId);

    // Special handling for Plex
    if (service.type === 'plex') {
      // Initial fetch with a small delay
      setTimeout(() => {
        Promise.all([
          fetchHealthStatus(service),
          fetchPlexSessions(service)
        ]).catch(console.error);
      }, Math.random() * 1000);

      // Set up separate Plex interval
      if (plexTimeoutRef.current) {
        clearTimeout(plexTimeoutRef.current);
      }
      plexTimeoutRef.current = setInterval(() => {
        fetchPlexSessions(service);
      }, PLEX_SESSIONS_INTERVAL);

      return;
    }

    // Initial fetch with a small delay for other services
    setTimeout(() => {
      Promise.all([
        fetchHealthStatus(service),
        fetchServiceStats(service)
      ]).catch(console.error);
    }, Math.random() * 1000);

    // Schedule background polling
    const interval = service.type === 'omegabrr' || service.type === 'tailscale' 
      ? HEALTH_CHECK_INTERVAL 
      : SERVICE_STATS_INTERVALS[service.type] || STATS_CHECK_INTERVAL;

    const timeoutId = setTimeout(() => {
      updateService(service);
    }, interval);

    updateTimeoutsRef.current.set(service.instanceId, timeoutId);
  }, [clearServiceTimeouts, fetchHealthStatus, fetchServiceStats, fetchPlexSessions]);

  useEffect(() => {
    if (!isAuthenticated || !configurations) {
      setServices(new Map());
      setIsLoading(false);
      return;
    }

    const newServices = new Map();
    Object.entries(configurations).forEach(([instanceId, config]) => {
      const [type] = instanceId.split('-');
      const template = serviceTemplates.find(t => t.type === type);
      const hasRequiredConfig = Boolean(config.url && config.apiKey);

      const service = {
        id: instanceId,
        instanceId,
        name: template?.name || 'Unknown Service',
        type: (template?.type || 'other') as ServiceType,
        status: hasRequiredConfig ? 'loading' as ServiceStatus : 'pending' as ServiceStatus,
        url: config.url,
        apiKey: config.apiKey,
        displayName: config.displayName,
        healthEndpoint: template?.healthEndpoint,
        message: hasRequiredConfig ? 'Loading service status' : 'Service not configured'
      } as Service;

      newServices.set(instanceId, service);
    });

    setServices(newServices);
    setIsLoading(false);

    // Initialize all services
    Array.from(newServices.values()).forEach(service => {
      updateService(service);
    });

    return () => {
      Array.from(newServices.keys()).forEach(serviceId => {
        clearServiceTimeouts(serviceId);
      });
      if (plexTimeoutRef.current) {
        clearTimeout(plexTimeoutRef.current);
      }
    };
  }, [configurations, isAuthenticated, updateService, clearServiceTimeouts]);

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

  const debouncedRefreshService = debounce(refreshService, 1000);

  return {
    services: Array.from(services.values()),
    isLoading,
    refreshService: debouncedRefreshService
  };
};
