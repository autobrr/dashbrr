/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, ReactNode, useCallback, useRef } from "react";
import { ServiceConfig } from "../types/service";
import { useAuth } from "../hooks/useAuth";
import { ConfigurationContext } from "./context";
import { ConfigurationContextType } from "./types";
import { api } from "../utils/api";

export function ConfigurationProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [configurations, setConfigurations] = useState<{
    [instanceId: string]: ServiceConfig;
  }>({});
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const configurationsRef = useRef(configurations);

  useEffect(() => {
    configurationsRef.current = configurations;
  }, [configurations]);

  const fetchConfigurations = useCallback(async () => {
    if (!isAuthenticated) {
      // Avoid an update loop: only clear if we actually had config.
      setConfigurations((prev) => (Object.keys(prev).length > 0 ? {} : prev));
      setIsLoading(false);
      return;
    }

    // Skip fetch if we already have configurations.
    if (Object.keys(configurationsRef.current).length > 0) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const data = await api.get<Record<string, ServiceConfig>>("/settings");
      setConfigurations(data);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch configurations";
      setError(errorMessage);
      console.error("Error fetching configurations:", err);
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    fetchConfigurations();
  }, [fetchConfigurations]);

  const updateConfiguration = async (
    instanceId: string,
    config: ServiceConfig
  ) => {
    try {
      setError(null);
      const updatedConfig = await api.post<ServiceConfig>(
        `/settings/${instanceId}`,
        config
      );

      // Update local state with the server response
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
      await api.delete(`/settings/${instanceId}`);

      // Remove from local state
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

  const contextValue: ConfigurationContextType = {
    configurations,
    updateConfiguration,
    deleteConfiguration,
    fetchConfigurations,
    isLoading,
    error,
  };

  return (
    <ConfigurationContext.Provider value={contextValue}>
      {children}
    </ConfigurationContext.Provider>
  );
}
