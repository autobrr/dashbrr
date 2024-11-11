/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, ReactNode, useCallback } from "react";
import { ServiceConfig } from "../types/service";
import { ConfigurationContext } from "./context";
import { ConfigurationContextType } from "./types";
import { useContext } from "react";

export function useConfiguration() {
  const context = useContext(ConfigurationContext);
  if (!context) {
    throw new Error(
      "useConfiguration must be used within a ConfigurationProvider"
    );
  }
  return context;
}

export function ConfigurationProvider({ children }: { children: ReactNode }) {
  const [configurations, setConfigurations] = useState<{
    [instanceId: string]: ServiceConfig;
  }>({});
  const [baseUrl, setBaseUrl] = useState<string>("/");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const buildUrl = useCallback(
    (path: string) => {
      // Remove any leading/trailing slashes from path
      const cleanPath = path.replace(/^\/+|\/+$/g, "");

      // Ensure path starts with api/
      const apiPath = cleanPath.startsWith("api/")
        ? cleanPath
        : `api/${cleanPath}`;

      // In development, return just the path to use Vite proxy
      if (import.meta.env.DEV) {
        return `/${apiPath}`;
      }

      // In production, use the origin and configured base URL
      const origin = window.location.origin;

      // Combine all parts ensuring no double slashes
      if (baseUrl && baseUrl !== "/") {
        return `${origin}${baseUrl}/${apiPath}`;
      }
      return `${origin}/${apiPath}`;
    },
    [baseUrl]
  );

  const getAuthHeaders = useCallback(() => {
    const accessToken = localStorage.getItem("access_token");
    return {
      "Content-Type": "application/json",
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    };
  }, []);

  const fetchBaseUrl = useCallback(async () => {
    try {
      // Use a direct path for the initial baseUrl fetch
      const response = await fetch("/api/settings/baseurl");
      if (!response.ok) {
        throw new Error(`Failed to fetch base URL: ${response.status}`);
      }

      const data = await response.json();
      const newBaseUrl = data.base_url || "/";
      setBaseUrl(newBaseUrl);

      // Set the global base URL for use in other parts of the application
      window.__BASE_URL__ = newBaseUrl;

      // Log the base URL for debugging
      console.log("Base URL set to:", newBaseUrl);
    } catch (err) {
      console.error("Error fetching base URL:", err);
      // Default to "/" if we can't fetch the base URL
      setBaseUrl("/");
      window.__BASE_URL__ = "/";
    }
  }, []);

  const fetchConfigurations = useCallback(async () => {
    const accessToken = localStorage.getItem("access_token");
    if (!accessToken) {
      setConfigurations({});
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch(buildUrl("settings"), {
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
  }, [buildUrl, getAuthHeaders]);

  useEffect(() => {
    fetchBaseUrl();
  }, [fetchBaseUrl]);

  const updateConfiguration = async (
    instanceId: string,
    config: ServiceConfig
  ) => {
    try {
      setError(null);
      const response = await fetch(buildUrl(`settings/${instanceId}`), {
        method: "POST",
        headers: getAuthHeaders(),
        body: JSON.stringify(config),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to update configuration");
      }

      const updatedConfig = await response.json();
      setConfigurations((prev) => ({
        ...prev,
        [instanceId]: updatedConfig,
      }));

      // Fetch fresh data to ensure consistency
      await fetchConfigurations();
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
      const response = await fetch(buildUrl(`settings/${instanceId}`), {
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

      // Fetch fresh data to ensure consistency
      await fetchConfigurations();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to delete configuration";
      setError(errorMessage);
      throw new Error(errorMessage);
    }
  };

  const contextValue: ConfigurationContextType = {
    configurations,
    baseUrl,
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
