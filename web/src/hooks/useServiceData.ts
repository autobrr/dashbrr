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
  useReducer,
} from "react";
import { Service } from "../types/service";
import { useConfiguration } from "../contexts/useConfiguration";
import { useAuth } from "./useAuth";
import { deriveHealthUpdate } from "./serviceData/merge";
import {
  initialServiceDataState,
  serviceDataReducer,
} from "./serviceData/reducer";

type ServiceDataContextValue = {
  services: Service[];
  isLoading: boolean;
  getService: (instanceId: string) => Service | undefined;
};

const ServiceDataContext = createContext<ServiceDataContextValue | undefined>(
  undefined
);

const useProvideServiceData = (): ServiceDataContextValue => {
  const { configurations } = useConfiguration();
  const { isAuthenticated } = useAuth();
  const [state, dispatch] = useReducer(
    serviceDataReducer,
    initialServiceDataState
  );

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);
  const latestPatchRef = useRef<Map<string, Partial<Service>>>(new Map());

  const applyHealthUpdate = useCallback(
    (payload: unknown) => {
      const update = deriveHealthUpdate(payload);
      if (!update) {
        return;
      }

      latestPatchRef.current.set(update.instanceId, update.patch);

      dispatch({
        type: "apply_patch",
        instanceId: update.instanceId,
        patch: update.patch,
      });
    },
    []
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
      dispatch({ type: "set_loading", isLoading: false });
      return;
    }

    connectSSE();
  }, [cleanupSSE, connectSSE, isAuthenticated]);

  useEffect(() => {
    if (!isAuthenticated) {
      latestPatchRef.current.clear();
      dispatch({ type: "reset" });
      return;
    }

    if (!configurations) {
      dispatch({ type: "set_loading", isLoading: true });
      return;
    }

    dispatch({
      type: "hydrate_configurations",
      configurations,
      latestPatchByInstance: new Map(latestPatchRef.current),
    });
  }, [configurations, isAuthenticated]);

  const getService = useCallback(
    (instanceId: string) => {
      return state.services.get(instanceId);
    },
    [state.services]
  );

  const servicesArray = useMemo(
    () => Array.from(state.services.values()),
    [state.services]
  );

  return {
    services: servicesArray,
    isLoading: state.isLoading,
    getService,
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
