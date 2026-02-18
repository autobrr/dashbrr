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

const hasOwn = (value: unknown, key: string): boolean =>
  typeof value === "object" &&
  value !== null &&
  Object.prototype.hasOwnProperty.call(value, key);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const mergeServicePayload = <T extends object>(
  current: T | undefined,
  incoming: T | undefined
): T | undefined => {
  if (!incoming) return current;
  if (!current) return { ...incoming };

  const merged = { ...current } as T;
  for (const key of Object.keys(incoming) as Array<keyof T>) {
    const nextValue = incoming[key];
    const prevValue = current[key];
    if (isRecord(prevValue) && isRecord(nextValue)) {
      (merged as Record<string, unknown>)[key as string] = {
        ...prevValue,
        ...nextValue,
      };
      continue;
    }
    merged[key] = nextValue;
  }

  return merged;
};

type HealthPatchPresence = {
  hasVersion: boolean;
  hasUpdateAvailable: boolean;
  hasResponseTime: boolean;
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
  const latestPatchRef = useRef<Map<string, Partial<Service>>>(new Map());

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
          stats: mergeServicePayload(cur.stats, partial.stats),
          details: mergeServicePayload(cur.details, partial.details),
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

  const servicePatchFromHealth = useCallback(
    (health: ServiceHealth, presence: HealthPatchPresence): Partial<Service> => {
      const patch: Partial<Service> = {
      status: health.status,
      message: health.message,
      lastChecked: health.lastChecked,
      stats: health.stats,
      details: health.details,
      health,
      };

      if (presence.hasResponseTime) {
        patch.responseTime = health.responseTime;
      }
      if (presence.hasVersion) {
        patch.version = health.version;
      }
      if (presence.hasUpdateAvailable) {
        patch.updateAvailable = health.updateAvailable;
      }
      if (health.message === "autobrr_releases") {
        patch.stats = undefined;
      }

      return patch;
    },
    []
  );

  const applyHealthUpdate = useCallback(
    (payload: unknown) => {
      if (typeof payload !== "object" || payload === null) {
        return;
      }

      const raw = payload as ServiceHealth;
      const instanceId = raw.serviceId;
      if (!instanceId) return;

      const lastChecked = parseDate((raw as unknown as { lastChecked?: unknown }).lastChecked);
      const health: ServiceHealth = {
        ...raw,
        lastChecked: lastChecked || new Date(),
      };

      const patch = servicePatchFromHealth(health, {
        hasVersion: hasOwn(payload, "version"),
        hasUpdateAvailable: hasOwn(payload, "updateAvailable"),
        hasResponseTime: hasOwn(payload, "responseTime"),
      });

      latestHealthRef.current.set(instanceId, health);
      latestPatchRef.current.set(instanceId, patch);
      updateService(instanceId, patch);

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
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as unknown;
        applyHealthUpdate(data);
      } catch (err) {
        console.error("SSE parse error:", err);
      }
    };

    es.onerror = () => {
      if (!mountedRef.current || eventSourceRef.current !== es) return;
      if (reconnectTimeoutRef.current) return;
      es.close();

      const retry = retryCountRef.current++;
      const delay = Math.min(1000 * Math.pow(2, retry), 30000);

      reconnectTimeoutRef.current = window.setTimeout(() => {
        reconnectTimeoutRef.current = null;
        if (
          mountedRef.current &&
          isAuthenticated &&
          eventSourceRef.current === es
        ) {
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
      latestPatchRef.current.clear();
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
          const patch = latestPatchRef.current.get(instanceId) ?? {};
          merged = {
            ...merged,
            ...patch,
            stats: mergeServicePayload(merged.stats, patch.stats),
            details: mergeServicePayload(merged.details, patch.details),
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
