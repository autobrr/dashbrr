/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  ReactNode,
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  AutobrrReleases,
  Service,
  ServiceConfig,
  ServiceHealth,
  ServiceStatus,
  ServiceType,
} from "../types/service";
import { useConfiguration } from "../contexts/useConfiguration";
import { useAuth } from "./useAuth";
import { serviceTemplates } from "../config/serviceTemplates";
import { api } from "../utils/api";

type RefreshKind = "health" | "stats" | "all";

type ServiceDataContextValue = {
  services: Service[];
  isLoading: boolean;
  getService: (instanceId: string) => Service | undefined;
  refreshService: (instanceId: string, kind?: RefreshKind) => Promise<void>;
};

const ServiceDataContext = createContext<ServiceDataContextValue | undefined>(
  undefined
);

const templateByType: Map<string, (typeof serviceTemplates)[number]> = new Map(
  serviceTemplates.map((t) => [t.type, t] as const)
);

const parseDate = (v: unknown): Date | undefined => {
  if (!v) return undefined;
  if (v instanceof Date) return v;
  if (typeof v === "string" || typeof v === "number") {
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? undefined : d;
  }
  return undefined;
};

const useProvideServiceData = (): ServiceDataContextValue => {
  const { configurations } = useConfiguration();
  const { isAuthenticated } = useAuth();

  const [services, setServices] = useState<Map<string, Service>>(new Map());
  const [isLoading, setIsLoading] = useState(true);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);
  const latestHealthRef = useRef<Map<string, ServiceHealth>>(new Map());

  const setServicesState = useCallback(
    (updater: (prev: Map<string, Service>) => Map<string, Service>) => {
      setServices((prev) => updater(prev));
    },
    []
  );

  const updateService = useCallback(
    (instanceId: string, partial: Partial<Service>) => {
      setServicesState((prev) => {
        const next = new Map(prev);
        const cur = next.get(instanceId);
        if (!cur) return next;

        const merged: Service = {
          ...cur,
          ...partial,
          stats: partial.stats ? { ...(cur.stats || {}), ...partial.stats } : cur.stats,
          details: partial.details ? { ...(cur.details || {}), ...partial.details } : cur.details,
        };

        next.set(instanceId, merged);
        return next;
      });
    },
    [setServicesState]
  );

  const serviceFromConfig = useCallback((instanceId: string, config: ServiceConfig): Service => {
    const [type] = instanceId.split("-");
    const template = templateByType.get(type);
    const hasRequiredConfig = Boolean(config.url);

    return {
      id: instanceId,
      instanceId,
      name: template?.name || "Unknown Service",
      type: (template?.type || "other") as ServiceType,
      status: (hasRequiredConfig ? "loading" : "pending") as ServiceStatus,
      url: config.url,
      accessUrl: config.accessUrl,
      apiKey: config.apiKey,
      displayName: config.displayName,
      healthEndpoint: template?.healthEndpoint,
      message: hasRequiredConfig ? "Waiting for updates" : "Service not configured",
      stats: {},
      details: {},
    };
  }, []);

  const servicePatchFromHealth = useCallback((health: ServiceHealth): Partial<Service> => {
    return {
      status: health.status,
      message: health.message,
      responseTime: health.responseTime,
      version: health.version,
      updateAvailable: health.updateAvailable,
      lastChecked: health.lastChecked,
      stats: health.stats,
      details: health.details,
      health,
    };
  }, []);

  const applyHealthUpdate = useCallback(
    (raw: ServiceHealth) => {
      const instanceId = raw.serviceId;
      if (!instanceId) return;

      const lastChecked = parseDate((raw as unknown as { lastChecked?: unknown }).lastChecked);
      const health: ServiceHealth = {
        ...raw,
        lastChecked: lastChecked || new Date(),
      };

      latestHealthRef.current.set(instanceId, health);
      updateService(instanceId, servicePatchFromHealth(health));

      if (health.message === "autobrr_releases" && health.stats?.autobrr) {
        const releases = health.stats.autobrr as unknown as AutobrrReleases;
        if (releases && Array.isArray(releases.data)) {
          updateService(instanceId, { releases });
        }
      }
    },
    [servicePatchFromHealth, updateService]
  );

  const cleanupSSE = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  const createEventSource = useCallback((url: string) => {
    try {
      return new EventSource(url, { withCredentials: true });
    } catch {
      return new EventSource(url);
    }
  }, []);

  const connectSSE = useCallback(() => {
    cleanupSSE();

    const es = createEventSource("/api/events");
    eventSourceRef.current = es;

    es.onopen = () => {
      retryCountRef.current = 0;
    };

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as ServiceHealth;
        applyHealthUpdate(data);
      } catch (err) {
        console.error("SSE parse error:", err);
      }
    };

    es.onerror = () => {
      if (!mountedRef.current) return;
      es.close();

      const retry = retryCountRef.current++;
      const delay = Math.min(1000 * Math.pow(2, retry), 30000);

      reconnectTimeoutRef.current = window.setTimeout(() => {
        if (mountedRef.current && isAuthenticated) {
          connectSSE();
        }
      }, delay);
    };
  }, [applyHealthUpdate, cleanupSSE, createEventSource, isAuthenticated]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      cleanupSSE();
    };
  }, [cleanupSSE]);

  useEffect(() => {
    if (!isAuthenticated) {
      cleanupSSE();
      setIsLoading(false);
      return;
    }

    connectSSE();
  }, [cleanupSSE, connectSSE, isAuthenticated]);

  useEffect(() => {
    if (!isAuthenticated) {
      setServicesState(() => new Map());
      latestHealthRef.current.clear();
      return;
    }

    if (!configurations) {
      setIsLoading(true);
      return;
    }

    const entries = Object.entries(configurations);
    const configuredIds = new Set(entries.map(([id]) => id));

    setServicesState((prev) => {
      const next = new Map(prev);

      for (const [instanceId, config] of entries) {
        const base = serviceFromConfig(instanceId, config);
        const existing = next.get(instanceId);
        let merged: Service = existing ? { ...existing, ...base } : base;

        const latestHealth = latestHealthRef.current.get(instanceId);
        if (latestHealth) {
          const patch = servicePatchFromHealth(latestHealth);
          merged = {
            ...merged,
            ...patch,
            stats: patch.stats ? { ...(merged.stats || {}), ...patch.stats } : merged.stats,
            details: patch.details ? { ...(merged.details || {}), ...patch.details } : merged.details,
          };

          if (latestHealth.message === "autobrr_releases" && latestHealth.stats?.autobrr) {
            const releases = latestHealth.stats.autobrr as unknown as AutobrrReleases;
            if (releases && Array.isArray(releases.data)) {
              merged = { ...merged, releases };
            }
          }
        }

        next.set(instanceId, merged);
      }

      for (const id of next.keys()) {
        if (!configuredIds.has(id)) next.delete(id);
      }

      return next;
    });

    setIsLoading(false);
  }, [configurations, isAuthenticated, serviceFromConfig, servicePatchFromHealth, setServicesState]);

  const refreshService = useCallback(async (instanceId: string, kind: RefreshKind = "all") => {
    try {
      await api.post(`/api/services/${instanceId}/refresh?kind=${kind}`);
    } catch (err) {
      console.error("Failed to refresh service:", err);
    }
  }, []);

  const getService = useCallback(
    (instanceId: string) => {
      return services.get(instanceId);
    },
    [services]
  );

  const servicesArray = useMemo(() => Array.from(services.values()), [services]);

  return {
    services: servicesArray,
    isLoading,
    getService,
    refreshService,
  };
};

export const ServiceDataProvider = ({ children }: { children: ReactNode }) => {
  const value = useProvideServiceData();
  return createElement(ServiceDataContext.Provider, { value }, children);
};

export const useServiceData = () => {
  const context = useContext(ServiceDataContext);
  if (!context) {
    throw new Error("useServiceData must be used within a ServiceDataProvider");
  }
  return context;
};
