/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, ReactNode, useCallback } from "react";
import { API_BASE_URL, API_PREFIX } from "../config/api";
import { ServiceConfig } from "../types/service";
import { useAuth } from "../hooks/useAuth";
import { ConfigurationContext } from "./context";
import { ConfigurationContextType } from "./types";

export function ConfigurationProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [configurations, setConfigurations] = useState<{
    [instanceId: string]: ServiceConfig;
  }>({});
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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

  const fetchConfigurations = useCallback(async () => {
    if (!isAuthenticated) {
      setConfigurations({});
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
      setConfigurations(data);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch configurations";
      setError(errorMessage);
      console.error("Error fetching configurations:", err);
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated, buildUrl, getAuthHeaders]);

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
      const response = await fetch(buildUrl(`/settings/${instanceId}`), {
        method: "DELETE",
        headers: getAuthHeaders(),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to delete configuration");
      }

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
