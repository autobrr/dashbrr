/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, ReactNode, useCallback, useRef } from "react";
import { API_BASE_URL, API_PREFIX } from "../config/api";
import { ServiceConfig } from "../types/service";
import { useAuth } from "./AuthContext";
import { ConfigurationContext } from "./context";
import { ConfigurationContextType } from "./types";

interface ConfigCache {
  data: { [instanceId: string]: ServiceConfig };
  timestamp: number;
}

const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes
const MAX_RETRIES = 3;
const RETRY_DELAY = 2000; // 2 seconds

export function ConfigurationProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [configurations, setConfigurations] = useState<{
    [instanceId: string]: ServiceConfig;
  }>({});
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const configCache = useRef<ConfigCache | null>(null);
  const retryCount = useRef(0);

  const buildUrl = useCallback((path: string) => {
    const apiPath = path.startsWith("/api") ? path : `${API_PREFIX}${path}`;
    return `${API_BASE_URL}${apiPath}`;
  }, []);

  const getAuthHeaders = useCallback(() => {
    const accessToken = localStorage.getItem("access_token");
    return {
      "Content-Type": "application/json",
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    };
  }, []);

  const fetchConfigurations = useCallback(
    async (retryAttempt = 0) => {
      if (!isAuthenticated) {
        setConfigurations({});
        setIsLoading(false);
        return;
      }

      // Check cache first
      if (
        configCache.current &&
        Date.now() - configCache.current.timestamp < CACHE_DURATION
      ) {
        setConfigurations(configCache.current.data);
        setIsLoading(false);
        return;
      }

      setIsLoading(true);
      setError(null);

      try {
        const response = await fetch(buildUrl("/settings"), {
          headers: getAuthHeaders(),
          cache: "no-store",
        });

        if (!response.ok) {
          throw new Error(`Failed to fetch configurations: ${response.status}`);
        }

        const data = await response.json();

        // Update cache and state
        configCache.current = {
          data,
          timestamp: Date.now(),
        };
        setConfigurations(data);
        retryCount.current = 0; // Reset retry count on success
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to fetch configurations";

        // Implement retry logic
        if (retryAttempt < MAX_RETRIES) {
          setTimeout(() => {
            fetchConfigurations(retryAttempt + 1);
          }, RETRY_DELAY * Math.pow(2, retryAttempt)); // Exponential backoff
          return;
        }

        setError(errorMessage);
        console.error("Error fetching configurations:", err);

        // Use cached data if available when all retries fail
        if (configCache.current) {
          setConfigurations(configCache.current.data);
          console.log("Using cached configuration data after fetch failure");
        }
      } finally {
        setIsLoading(false);
      }
    },
    [isAuthenticated, buildUrl, getAuthHeaders]
  );

  useEffect(() => {
    fetchConfigurations();
  }, [fetchConfigurations]);

  const updateConfiguration = async (
    instanceId: string,
    config: ServiceConfig
  ) => {
    try {
      setError(null);
      const response = await fetch(buildUrl(`/settings/${instanceId}`), {
        method: "POST",
        headers: getAuthHeaders(),
        body: JSON.stringify(config),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to update configuration");
      }

      const updatedConfig = await response.json();

      // Update both cache and state
      if (configCache.current) {
        configCache.current.data = {
          ...configCache.current.data,
          [instanceId]: updatedConfig,
        };
        configCache.current.timestamp = Date.now();
      }

      setConfigurations((prev) => ({
        ...prev,
        [instanceId]: updatedConfig,
      }));

      return updatedConfig;
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to update configuration";
      setError(errorMessage);
      throw new Error(errorMessage);
    }
  };

  const deleteConfiguration = async (instanceId: string) => {
    try {
      setError(null);
      const response = await fetch(buildUrl(`/settings/${instanceId}`), {
        method: "DELETE",
        headers: getAuthHeaders(),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to delete configuration");
      }

      // Update both cache and state
      if (configCache.current) {
        const newCacheData = { ...configCache.current.data };
        delete newCacheData[instanceId];
        configCache.current = {
          data: newCacheData,
          timestamp: Date.now(),
        };
      }

      setConfigurations((prev) => {
        const newConfigs = { ...prev };
        delete newConfigs[instanceId];
        return newConfigs;
      });
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to delete configuration";
      setError(errorMessage);
      throw new Error(errorMessage);
    }
  };

  const forceRefresh = useCallback(async () => {
    // Clear cache and fetch fresh data
    configCache.current = null;
    await fetchConfigurations();
  }, [fetchConfigurations]);

  const contextValue: ConfigurationContextType = {
    configurations,
    updateConfiguration,
    deleteConfiguration,
    fetchConfigurations: forceRefresh, // Use forceRefresh for manual refreshes
    isLoading,
    error,
  };

  return (
    <ConfigurationContext.Provider value={contextValue}>
      {children}
    </ConfigurationContext.Provider>
  );
}
