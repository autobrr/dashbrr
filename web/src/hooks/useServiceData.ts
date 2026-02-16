/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

const parseDate = (v: unknown): Date | undefined => {
  if (!v) return undefined;
  if (v instanceof Date) return v;
  if (typeof v === "string" || typeof v === "number") {
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? undefined : d;
  }
  return undefined;
};

export const useServiceData = () => {
  const { configurations } = useConfiguration();
  const { isAuthenticated } = useAuth();

  const [services, setServices] = useState<Map<string, Service>>(new Map());
  const [isLoading, setIsLoading] = useState(true);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);

  const setServicesState = useCallback(
    (updater: (prev: Map<string, Service>) => Map<string, Service>) => {
      setServices((prev) => {
        const next = updater(prev);
        return next;
      });
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
          // shallow-merge nested bags
          stats: partial.stats ? { ...(cur.stats || {}), ...partial.stats } : cur.stats,
          details: partial.details
            ? { ...(cur.details || {}), ...partial.details }
            : cur.details,
        };

        next.set(instanceId, merged);
        return next;
      });
    },
    [setServicesState]
  );

  const upsertFromConfig = useCallback(
    (instanceId: string, config: ServiceConfig) => {
      const [type] = instanceId.split("-");
      const template = serviceTemplates.find((t) => t.type === type);
      // API keys are write-only; only URL presence is known client-side.
      const hasRequiredConfig = Boolean(config.url);

      const base: Service = {
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

      setServicesState((prev) => {
        const next = new Map(prev);
        const existing = next.get(instanceId);
        next.set(instanceId, existing ? { ...existing, ...base } : base);
        return next;
      });
    },
    [setServicesState]
  );

  const applyHealthUpdate = useCallback(
    (raw: ServiceHealth) => {
      const instanceId = raw.serviceId;
      if (!instanceId) return;

      const lastChecked = parseDate((raw as unknown as { lastChecked?: unknown }).lastChecked);
      const health: ServiceHealth = {
        ...raw,
        lastChecked: lastChecked || new Date(),
      };

      updateService(instanceId, {
        status: health.status,
        message: health.message,
        responseTime: health.responseTime,
        version: health.version,
        updateAvailable: health.updateAvailable,
        lastChecked: health.lastChecked,
        stats: health.stats,
        details: health.details,
        health,
      });

      if (health.message === "autobrr_releases" && health.stats?.autobrr) {
        const releases = health.stats.autobrr as unknown as AutobrrReleases;
        if (releases && Array.isArray(releases.data)) {
          updateService(instanceId, { releases });
        }
      }
    },
    [updateService]
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

  const connectSSE = useCallback(() => {
    cleanupSSE();

    const es = new EventSource("/api/events");
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

      // backoff: 1s, 2s, 4s, ... max 30s
      const retry = retryCountRef.current++;
      const delay = Math.min(1000 * Math.pow(2, retry), 30000);

      reconnectTimeoutRef.current = window.setTimeout(() => {
        if (mountedRef.current && isAuthenticated) {
          connectSSE();
        }
      }, delay);
    };
  }, [applyHealthUpdate, cleanupSSE, isAuthenticated]);

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
      setServicesState(() => new Map());
      setIsLoading(false);
      return;
    }

    if (!configurations) {
      setIsLoading(true);
      return;
    }

    // Upsert configured services
    for (const [instanceId, config] of Object.entries(configurations)) {
      upsertFromConfig(instanceId, config);
    }

    // Remove deleted services
    const configuredIds = new Set(Object.keys(configurations));
    setServicesState((prev) => {
      const next = new Map(prev);
      for (const id of next.keys()) {
        if (!configuredIds.has(id)) next.delete(id);
      }
      return next;
    });

    connectSSE();
    setIsLoading(false);
  }, [configurations, connectSSE, cleanupSSE, isAuthenticated, setServicesState, upsertFromConfig]);

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
