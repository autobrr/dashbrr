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
  AutobrrReleases,
  MaintainerrCollection,
  PlexSession,
  OverseerrStats,
  SonarrQueue,
  RadarrQueue,
  ProwlarrStats,
  ProwlarrIndexer,
  ServiceConfig,
  ServiceHealth} from '../types/service';
import { useConfiguration } from '../contexts/useConfiguration';
import { useAuth } from '../contexts/AuthContext';
import serviceTemplates from '../config/serviceTemplates';
import { api } from '../utils/api';
import { cache, CACHE_PREFIXES } from '../utils/cache';

interface ServiceData {
  stats?: ServiceStats;
  details?: ServiceDetails;
  releases?: AutobrrReleases;
}

// Background polling intervals
const STATS_CHECK_INTERVAL = 300000;   // 5 minutes for service stats
const PLEX_SESSIONS_INTERVAL = 5000;   // 5 seconds for Plex sessions (fallback)
const OVERSEERR_REQUESTS_INTERVAL = 30000; // 30 seconds for Overseerr requests (fallback)
const RADARR_QUEUE_INTERVAL = 30000;    // 30 seconds for Radarr queue (fallback)
const SONARR_QUEUE_INTERVAL = 30000;    // 30 seconds for Sonarr queue (fallback)
const PROWLARR_STATS_INTERVAL = 60000;  // 1 minute for Prowlarr stats (fallback)
const AUTOBRR_INTERVAL = 60000;         // 30 seconds for autobrr (fallback)

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
  const isInitialLoadRef = useRef(true);
  const eventSourceRef = useRef<EventSource | null>(null);

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
          } : currentService.details,
          releases: data.releases || currentService.releases
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

  const fetchOverseerrRequests = useCallback(async (service: Service) => {
    try {
      const stats = await api.get<OverseerrStats>(
        `/api/overseerr/requests?instanceId=${service.instanceId}`
      );
      const data = {
        stats: { overseerr: stats },
        details: {
          overseerr: {
            pendingCount: stats.pendingCount,
            totalRequests: stats.requests.length
          }
        }
      };
      updateServiceData(service.instanceId, data);
    } catch (error) {
      console.error('Error fetching Overseerr requests:', error);
    }
  }, [updateServiceData]);

  const fetchRadarrQueue = useCallback(async (service: Service) => {
    try {
      const queueData = await api.get<RadarrQueue>(
        `/api/radarr/queue?instanceId=${service.instanceId}`
      );
      if (queueData) {
        const downloadingCount = queueData.records.filter(r => r.status === 'downloading').length;
        const totalSize = queueData.records.reduce((acc, r) => acc + r.size, 0);
        
        const data = {
          stats: { radarr: { queue: queueData } },
          details: {
            radarr: {
              queueCount: queueData.totalRecords,
              totalRecords: queueData.totalRecords,
              downloadingCount,
              totalSize
            }
          }
        };
        updateServiceData(service.instanceId, data);
      }
    } catch (error) {
      console.error('Error fetching Radarr queue:', error);
    }
  }, [updateServiceData]);

  const fetchSonarrQueue = useCallback(async (service: Service) => {
    try {
      const queueData = await api.get<SonarrQueue>(
        `/api/sonarr/queue?instanceId=${service.instanceId}`
      );
      if (queueData) {
        const downloadingCount = queueData.records.filter(r => r.status === 'downloading').length;
        const episodeCount = queueData.records.reduce((acc, r) => acc + r.episodes.length, 0);
        const totalSize = queueData.records.reduce((acc, r) => acc + r.size, 0);
        
        const data = {
          stats: { sonarr: { queue: queueData } },
          details: {
            sonarr: {
              queueCount: queueData.totalRecords,
              monitored: 0,
              totalRecords: queueData.totalRecords,
              downloadingCount,
              episodeCount,
              totalSize
            }
          }
        };
        updateServiceData(service.instanceId, data);
      }
    } catch (error) {
      console.error('Error fetching Sonarr queue:', error);
    }
  }, [updateServiceData]);

  const fetchServiceStats = useCallback(async (service: Service) => {
    if (service.type === 'omegabrr' || service.type === 'tailscale' || service.type === 'general') return;
    if (!service.url || !service.apiKey) return;

    if (service.type === 'plex') {
      await fetchPlexSessions(service);
      return;
    }
    if (service.type === 'overseerr') {
      await fetchOverseerrRequests(service);
      return;
    }
    if (service.type === 'radarr') {
      await fetchRadarrQueue(service);
      return;
    }
    if (service.type === 'sonarr') {
      await fetchSonarrQueue(service);
      return;
    }

    const statsCacheKey = `${CACHE_PREFIXES.STATS}${service.instanceId}`;
    const { data: cachedStats, isStale } = cache.get<ServiceData>(statsCacheKey);

    if (cachedStats && !isStale) {
      updateServiceData(service.instanceId, cachedStats);
      return;
    }

    let data: ServiceData | undefined;

    try {
      switch (service.type) {
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
          const [statsData, ircData, releasesData] = await Promise.all([
            api.get<AutobrrStats>(`/api/autobrr/stats?instanceId=${service.instanceId}`),
            api.get<AutobrrIRC[]>(`/api/autobrr/irc?instanceId=${service.instanceId}`),
            api.get<AutobrrReleases>(`/api/autobrr/releases?instanceId=${service.instanceId}`)
          ]);
          if (statsData && ircData) {
            data = { 
              stats: { autobrr: statsData }, 
              details: { autobrr: { 
                irc: ircData,
                base_url: service.url || ''
              }},
              releases: releasesData
            };
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
  }, [updateServiceData, fetchPlexSessions, fetchOverseerrRequests, fetchRadarrQueue, fetchSonarrQueue]);

  const fetchHealthStatus = useCallback(async (service: Service) => {
    const healthCacheKey = `${CACHE_PREFIXES.HEALTH}${service.instanceId}`;
    const { data: cachedHealth, isStale } = cache.get<ServiceHealth>(healthCacheKey);

    if (cachedHealth && !isStale) {
      updateServiceData(service.instanceId, cachedHealth);
      return;
    }

    try {
      const health = await api.get<ServiceHealth>(`/api/health/${service.instanceId}`);
      if (health) {
        if (service.type === 'plex' && health.message === 'plex_sessions' && health.stats?.plex?.sessions) {
          const sessions = health.stats.plex.sessions;
          updateServiceData(service.instanceId, {
            stats: { plex: { sessions } },
            details: {
              plex: {
                activeStreams: sessions.length,
                transcoding: sessions.filter((s: PlexSession) => s.TranscodeSession).length
              }
            }
          });
        } else if (service.type === 'overseerr' && health.message === 'overseerr_requests' && health.stats?.overseerr) {
          const requests = health.stats.overseerr;
          updateServiceData(service.instanceId, {
            stats: { overseerr: requests },
            details: {
              overseerr: {
                pendingCount: requests.pendingCount,
                totalRequests: requests.requests.length
              }
            }
          });
        } else if (service.type === 'radarr' && health.message === 'radarr_queue' && health.stats?.radarr?.queue) {
          const queue = health.stats.radarr.queue;
          const downloadingCount = queue.records.filter(r => r.status === 'downloading').length;
          const totalSize = queue.records.reduce((acc, r) => acc + r.size, 0);
          
          updateServiceData(service.instanceId, {
            stats: { radarr: { queue } },
            details: {
              radarr: {
                queueCount: queue.totalRecords,
                totalRecords: queue.totalRecords,
                downloadingCount,
                totalSize
              }
            }
          });
        } else if (service.type === 'sonarr' && health.message === 'sonarr_queue' && health.stats?.sonarr?.queue) {
          const queue = health.stats.sonarr.queue;
          const downloadingCount = queue.records.filter(r => r.status === 'downloading').length;
          const episodeCount = queue.records.reduce((acc, r) => acc + r.episodes.length, 0);
          const totalSize = queue.records.reduce((acc, r) => acc + r.size, 0);
          
          updateServiceData(service.instanceId, {
            stats: { sonarr: { queue } },
            details: {
              sonarr: {
                queueCount: queue.totalRecords,
                monitored: 0,
                totalRecords: queue.totalRecords,
                downloadingCount,
                episodeCount,
                totalSize
              }
            }
          });
        } else if (service.type === 'sonarr' && health.message === 'sonarr_stats' && health.stats?.sonarr) {
          const currentService = services.get(service.instanceId);
          const sonarrStats = health.stats.sonarr;
          const currentQueue = currentService?.stats?.sonarr?.queue || { totalRecords: 0, records: [] };
          
          updateServiceData(service.instanceId, {
            stats: { 
              sonarr: {
                queue: currentQueue,
                stats: sonarrStats.stats,
                version: sonarrStats.version
              }
            },
            details: {
              sonarr: {
                queueCount: currentService?.details?.sonarr?.queueCount || 0,
                monitored: sonarrStats.stats?.monitored || 0,
                version: sonarrStats.version
              }
            }
          });
        } else if (service.type === 'prowlarr' && health.message === 'prowlarr_stats' && health.stats?.prowlarr?.stats) {
          const currentService = services.get(service.instanceId);
          const prowlarrStats = health.stats.prowlarr.stats as ProwlarrStats;
          const currentIndexers = currentService?.stats?.prowlarr?.indexers || [];
          const currentIndexerStats = currentService?.stats?.prowlarr?.prowlarrIndexerStats || {
            id: 1,
            indexers: []
          };
          
          updateServiceData(service.instanceId, {
            stats: { 
              prowlarr: {
                stats: prowlarrStats,
                indexers: currentIndexers,
                prowlarrIndexerStats: currentIndexerStats
              }
            },
            details: {
              prowlarr: {
                activeIndexers: currentIndexers.filter(i => i.enable).length,
                totalGrabs: prowlarrStats.grabCount
              }
            }
          });
        } else if (service.type === 'prowlarr' && health.message === 'prowlarr_indexers' && health.stats?.prowlarr?.indexers) {
          const currentService = services.get(service.instanceId);
          const prowlarrIndexers = health.stats.prowlarr.indexers;
          const currentStats = currentService?.stats?.prowlarr?.stats as ProwlarrStats;
          const currentIndexerStats = currentService?.stats?.prowlarr?.prowlarrIndexerStats || {
            id: 1,
            indexers: []
          };
          
          updateServiceData(service.instanceId, {
            stats: { 
              prowlarr: {
                stats: currentStats,
                indexers: prowlarrIndexers,
                prowlarrIndexerStats: currentIndexerStats
              }
            },
            details: {
              prowlarr: {
                activeIndexers: prowlarrIndexers.filter(i => i.enable).length,
                totalGrabs: currentStats?.grabCount || 0
              }
            }
          });
        } else {
          cache.set(healthCacheKey, health);
          updateServiceData(service.instanceId, health);
        }
      }
    } catch (error) {
      console.error(`Error fetching health for ${service.type}:`, error);
    }
  }, [updateServiceData, services]);

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

    Promise.all([
      fetchHealthStatus(service),
      fetchServiceStats(service)
    ]).catch(console.error);

    const interval = service.type === 'plex' ? PLEX_SESSIONS_INTERVAL :
                    service.type === 'overseerr' ? OVERSEERR_REQUESTS_INTERVAL :
                    service.type === 'radarr' ? RADARR_QUEUE_INTERVAL :
                    service.type === 'sonarr' ? SONARR_QUEUE_INTERVAL :
                    service.type === 'prowlarr' ? PROWLARR_STATS_INTERVAL :
                    service.type === 'autobrr' ? AUTOBRR_INTERVAL :
                    STATS_CHECK_INTERVAL;

    const timeoutId = setTimeout(() => {
      updateService(service);
    }, interval);

    updateTimeoutsRef.current.set(service.instanceId, timeoutId);
  }, [clearServiceTimeout, fetchHealthStatus, fetchServiceStats]);

  const initializeSSE = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const eventSource = new EventSource('/api/events');

    eventSource.onmessage = (event) => {
      try {
        const health = JSON.parse(event.data) as ServiceHealth;
        
        switch (health.message) {
          case 'plex_sessions': {
            if (health.stats?.plex?.sessions) {
              const sessions = health.stats.plex.sessions;
              updateServiceData(health.serviceId, {
                stats: { plex: { sessions } },
                details: {
                  plex: {
                    activeStreams: sessions.length,
                    transcoding: sessions.filter((s: PlexSession) => s.TranscodeSession).length
                  }
                }
              });
            }
            break;
          }
          case 'autobrr_irc_status': {
            if (health.details?.autobrr?.irc) {
              const ircStatus = health.details.autobrr.irc as AutobrrIRC[];
              const currentService = services.get(health.serviceId);
              updateServiceData(health.serviceId, {
                details: {
                  autobrr: {
                    irc: ircStatus,
                    base_url: currentService?.url || ''
                  }
                }
              });
            }
            break;
          }
          case 'autobrr_releases': {
            if (health.stats?.autobrr && 'data' in health.stats.autobrr) {
              const releases = health.stats.autobrr as unknown as AutobrrReleases;
              updateServiceData(health.serviceId, {
                releases
              });
            }
            break;
          }
          case 'autobrr_stats': {
            if (health.stats?.autobrr) {
              const stats = health.stats.autobrr as AutobrrStats;
              updateServiceData(health.serviceId, {
                stats: { autobrr: stats }
              });
            }
            break;
          }
          case 'overseerr_requests': {
            if (health.stats?.overseerr) {
              const stats = health.stats.overseerr;
              updateServiceData(health.serviceId, {
                stats: { overseerr: stats },
                details: {
                  overseerr: {
                    pendingCount: stats.pendingCount,
                    totalRequests: stats.requests.length
                  }
                }
              });
            }
            break;
          }
          case 'radarr_queue': {
            if (health.stats?.radarr?.queue) {
              const queue = health.stats.radarr.queue;
              const downloadingCount = queue.records.filter(r => r.status === 'downloading').length;
              const totalSize = queue.records.reduce((acc, r) => acc + r.size, 0);
              
              updateServiceData(health.serviceId, {
                stats: { radarr: { queue } },
                details: {
                  radarr: {
                    queueCount: queue.totalRecords,
                    totalRecords: queue.totalRecords,
                    downloadingCount,
                    totalSize
                  }
                }
              });
            }
            break;
          }
          case 'sonarr_queue': {
            if (health.stats?.sonarr?.queue) {
              const queue = health.stats.sonarr.queue;
              const downloadingCount = queue.records.filter(r => r.status === 'downloading').length;
              const episodeCount = queue.records.reduce((acc, r) => acc + r.episodes.length, 0);
              const totalSize = queue.records.reduce((acc, r) => acc + r.size, 0);
              
              updateServiceData(health.serviceId, {
                stats: { sonarr: { queue } },
                details: {
                  sonarr: {
                    queueCount: queue.totalRecords,
                    monitored: 0,
                    totalRecords: queue.totalRecords,
                    downloadingCount,
                    episodeCount,
                    totalSize
                  }
                }
              });
            }
            break;
          }
          case 'sonarr_stats': {
            if (health.stats?.sonarr) {
              const currentService = services.get(health.serviceId);
              const sonarrStats = health.stats.sonarr;
              const currentQueue = currentService?.stats?.sonarr?.queue || { totalRecords: 0, records: [] };
              
              updateServiceData(health.serviceId, {
                stats: { 
                  sonarr: {
                    queue: currentQueue,
                    stats: sonarrStats.stats,
                    version: sonarrStats.version
                  }
                },
                details: {
                  sonarr: {
                    queueCount: currentService?.details?.sonarr?.queueCount || 0,
                    monitored: sonarrStats.stats?.monitored || 0,
                    version: sonarrStats.version
                  }
                }
              });
            }
            break;
          }
          case 'prowlarr_stats': {
            if (health.stats?.prowlarr?.stats) {
              const currentService = services.get(health.serviceId);
              const prowlarrStats = health.stats.prowlarr.stats as ProwlarrStats;
              const currentIndexers = currentService?.stats?.prowlarr?.indexers || [];
              const currentIndexerStats = currentService?.stats?.prowlarr?.prowlarrIndexerStats || {
                id: 1,
                indexers: []
              };
              
              updateServiceData(health.serviceId, {
                stats: { 
                  prowlarr: {
                    stats: prowlarrStats,
                    indexers: currentIndexers,
                    prowlarrIndexerStats: currentIndexerStats
                  }
                },
                details: {
                  prowlarr: {
                    activeIndexers: currentIndexers.filter(i => i.enable).length,
                    totalGrabs: prowlarrStats.grabCount
                  }
                }
              });
            }
            break;
          }
          case 'prowlarr_indexers': {
            if (health.stats?.prowlarr?.indexers) {
              const currentService = services.get(health.serviceId);
              const prowlarrIndexers = health.stats.prowlarr.indexers;
              const currentStats = currentService?.stats?.prowlarr?.stats as ProwlarrStats;
              const currentIndexerStats = currentService?.stats?.prowlarr?.prowlarrIndexerStats || {
                id: 1,
                indexers: []
              };
              
              updateServiceData(health.serviceId, {
                stats: { 
                  prowlarr: {
                    stats: currentStats,
                    indexers: prowlarrIndexers,
                    prowlarrIndexerStats: currentIndexerStats
                  }
                },
                details: {
                  prowlarr: {
                    activeIndexers: prowlarrIndexers.filter(i => i.enable).length,
                    totalGrabs: currentStats?.grabCount || 0
                  }
                }
              });
            }
            break;
          }
          default: {
            if (health.serviceId) {
              updateServiceData(health.serviceId, health);
            }
          }
        }
      } catch (error) {
        console.error('Error processing SSE message:', error);
      }
    };

    eventSource.onerror = (error) => {
      console.error('SSE connection error:', error);
      eventSource.close();
      setTimeout(initializeSSE, 5000);
    };

    eventSourceRef.current = eventSource;
  }, [updateServiceData, services]);

  useEffect(() => {
    if (!isAuthenticated || !configurations) {
      setServices(new Map());
      setIsLoading(false);
      return;
    }

    const currentPlexTimeout = plexTimeoutRef.current;
    const configHash = JSON.stringify(configurations);
    
    if (configHash === configHashRef.current && !isInitialLoadRef.current) {
      return;
    }
    
    configHashRef.current = configHash;
    initializeSSE();

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
    isInitialLoadRef.current = false;

    return () => {
      Array.from(services.keys()).forEach(clearServiceTimeout);
      if (currentPlexTimeout) {
        clearTimeout(currentPlexTimeout);
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, [configurations, isAuthenticated, clearServiceTimeout, initializeService, updateService, services, initializeSSE]);

  const refreshService = useCallback((instanceId: string, refreshType: 'health' | 'stats' | 'all' = 'all'): void => {
    const service = services.get(instanceId);
    if (!service) return;
    
    if (refreshType === 'all' || refreshType === 'health') {
      cache.remove(`${CACHE_PREFIXES.HEALTH}${instanceId}`);
    }
    if (refreshType === 'all' || refreshType === 'stats') {
      cache.remove(`${CACHE_PREFIXES.STATS}${instanceId}`);
    }
    
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
