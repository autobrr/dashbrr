/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState } from "react";
import { useConfiguration } from "../../contexts/useConfiguration";
import { ServiceConfig } from "../../types/service";
import { Button } from "../ui/Button";
import { FormInput } from "../ui/FormInput";
import { toast } from "react-hot-toast";
import { api } from "../../utils/api";
import { useServiceHealth } from "../../hooks/useServiceHealth";

interface ConfigurationFormProps {
  serviceName: string;
  instanceId: string;
  displayName: string;
  onClose: () => void;
}

export const ConfigurationForm = ({
  instanceId,
  displayName: initialDisplayName,
  onClose,
}: ConfigurationFormProps) => {
  const { configurations, updateConfiguration, fetchConfigurations } =
    useConfiguration();
  const { refreshServiceHealth } = useServiceHealth();
  const currentConfig = configurations[instanceId];
  const serviceType = instanceId.split("-")[0];

  const [url, setUrl] = useState(currentConfig?.url || "");
  const [accessUrl, setAccessUrl] = useState(currentConfig?.accessUrl || "");
  const [apiKey, setApiKey] = useState(currentConfig?.apiKey || "");
  const [displayName, setDisplayName] = useState(
    currentConfig?.displayName || initialDisplayName
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validateConfiguration = async (config: ServiceConfig) => {
    try {
      const queryParams = new URLSearchParams({
        url: config.url || "",
        ...(config.apiKey && { apiKey: config.apiKey }),
      }).toString();

      const health = await api.get<{
        status: string;
        message?: string;
      }>(`/api/health/${instanceId}?${queryParams}`);

      if (health.status === "error" || health.status === "offline") {
        throw new Error(health.message || "Failed to validate configuration");
      }

      return true;
    } catch (err) {
      console.error("Validation error:", err);
      if (err instanceof Error) {
        throw err;
      }
      throw new Error("Failed to validate configuration");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      const config: ServiceConfig = {
        url: url.endsWith("/") ? url.slice(0, -1) : url,
        accessUrl: accessUrl
          ? accessUrl.endsWith("/")
            ? accessUrl.slice(0, -1)
            : accessUrl
          : undefined,
        displayName,
        ...(serviceType !== "general" ? { apiKey } : {}),
      };

      // Validate configuration
      if (currentConfig) {
        await validateConfiguration(config);
      }

      // Update the configuration
      await updateConfiguration(instanceId, config);

      // Immediately refresh the health status
      await refreshServiceHealth(instanceId);

      // Force a refresh of all configurations to ensure UI is up to date
      await fetchConfigurations();

      toast.success("Configuration saved successfully");
      onClose();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to update configuration";
      if (errorMessage.includes("API token invalid")) {
        toast.error("Invalid API token. Please check your credentials.");
      } else {
        toast.error(errorMessage);
      }
      setError(errorMessage);
    } finally {
      setIsSubmitting(false);
    }
  };

  const getApiKeyLabel = () => {
    switch (serviceType) {
      case "plex":
        return "X-Plex-Token";
      case "radarr":
      case "sonarr":
      case "prowlarr":
        return "API Key";
      case "overseerr":
        return "API Key";
      default:
        return "API Key";
    }
  };

  const getSettingsUrl = (path: string): string | null => {
    if (!url) return null;
    const baseUrl = accessUrl || url;
    return `${baseUrl}${path}`;
  };

  const getApiKeyHelp = () => {
    switch (serviceType) {
      case "autobrr":
        return {
          prefix: "Found in ",
          text: "Settings > API",
          link: getSettingsUrl("/settings/api"),
        };
      case "omegabrr":
        return {
          prefix: "Found in ",
          text: "config.toml",
          link: null,
        };
      case "plex":
        return {
          prefix: "Get your ",
          text: "X-Plex-Token",
          link: "https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/",
        };
      case "radarr":
      case "sonarr":
      case "prowlarr":
        return {
          prefix: "Found in ",
          text: "Settings > General",
          link: getSettingsUrl("/settings/general"),
        };
      case "overseerr":
        return {
          prefix: "Found in ",
          text: "Settings",
          link: getSettingsUrl("/settings/main"),
        };
      default:
        return {
          prefix: "",
          text: "",
          link: null,
        };
    }
  };

  const getUrlPlaceholder = () => {
    switch (serviceType) {
      case "plex":
        return "http://localhost:32400";
      case "general":
        return "Enter full URL including health endpoint";
      default:
        return "Enter service URL";
    }
  };

  const apiKeyHelp = getApiKeyHelp();

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <FormInput
        id="displayName"
        label="Display Name"
        type="text"
        value={displayName}
        onChange={(e) => setDisplayName(e.target.value)}
        placeholder="Enter display name"
        required
      />

      <FormInput
        id="url"
        label="URL"
        type="text"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        placeholder={getUrlPlaceholder()}
        helpText={{
          prefix: "Used for ",
          text: "API communication and health checks",
          link: null,
        }}
        required
        data-1p-ignore
      />

      <FormInput
        id="accessUrl"
        label="Access URL (Optional)"
        type="text"
        value={accessUrl}
        onChange={(e) => setAccessUrl(e.target.value)}
        placeholder="Leave empty to use main URL"
        helpText={{
          prefix: "Override ",
          text: "URL used when opening service in browser. Useful for internal/external URL differences.",
          link: null,
        }}
        data-1p-ignore
      />

      {serviceType !== "general" && (
        <FormInput
          id="apiKey"
          label={getApiKeyLabel()}
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder={`Enter ${getApiKeyLabel()}`}
          helpText={apiKeyHelp}
          required
          data-1p-ignore
        />
      )}

      {error && (
        <div className="text-red-600 dark:text-red-400 text-sm">{error}</div>
      )}

      <div className="flex justify-end space-x-3">
        <Button
          type="button"
          variant="secondary"
          onClick={onClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving..." : "Save"}
        </Button>
      </div>
    </form>
  );
};
