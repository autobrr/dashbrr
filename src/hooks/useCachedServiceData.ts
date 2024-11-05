import { useCallback } from 'react';
import { Service, ServiceType } from '../types/service';
import { useConfiguration } from '../contexts/useConfiguration';
import serviceTemplates from '../config/serviceTemplates';
import { cache } from '../utils/cache';

const CACHE_KEY = 'cached_services';
const CACHE_TTL = 300000; // 5 minutes

interface CacheData {
  services: Service[];
  timestamp: number;
}

export function useCachedServiceData() {
  const { configurations } = useConfiguration();

  const getServices = useCallback((): Service[] => {
    // Try to get from cache first
    const cachedData = cache.get<CacheData>(CACHE_KEY);
    if (cachedData?.data?.services && (Date.now() - (cachedData.data.timestamp || 0) < CACHE_TTL)) {
      return cachedData.data.services;
    }

    if (!configurations) return [];

    const services = Object.entries(configurations).map(([instanceId, config]) => {
      const [type] = instanceId.split('-');
      const template = serviceTemplates.find(t => t.type === type);
      const hasRequiredConfig = Boolean(config.url && config.apiKey);

      return {
        id: instanceId,
        instanceId,
        name: template?.name || 'Unknown Service',
        type: (template?.type || 'other') as ServiceType,
        status: hasRequiredConfig ? 'loading' : 'pending',
        url: config.url,
        apiKey: config.apiKey,
        displayName: config.displayName,
        healthEndpoint: template?.healthEndpoint,
        message: hasRequiredConfig ? 'Loading service status' : 'Service not configured'
      } as Service;
    });

    // Update cache
    cache.set(CACHE_KEY, { services, timestamp: Date.now() });
    return services;
  }, [configurations]);

  const services = getServices();
  const isLoading = !configurations;

  return {
    services,
    isLoading,
    refresh: () => {
      cache.remove(CACHE_KEY);
      return getServices();
    },
  };
}
