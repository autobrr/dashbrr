/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useState } from "react";
import { useConfiguration } from "../../contexts/useConfiguration";
import { CustomServiceConfig, ServiceConfig } from "../../types/service";
import { Button } from "../ui/Button";
import { FormInput } from "../ui/FormInput";
import { toast } from "react-hot-toast";
import { api } from "../../utils/api";
import { usePlexPinAuth } from "../../hooks/usePlexPinAuth";
import { getGeneralConfig, saveGeneralConfig } from "../../api/general";
import { validateCustomServiceConfig } from "./general/customServiceConfig";
import { CustomServiceSection } from "./general/CustomServiceSection";

interface ConfigurationFormProps {
  instanceId: string;
  displayName: string;
  onClose: () => void;
}

export const ConfigurationForm = ({
  instanceId,
  displayName: initialDisplayName,
  onClose,
}: ConfigurationFormProps) => {
  const { configurations, updateConfiguration } = useConfiguration();
  const currentConfig = configurations[instanceId];
  const hasExistingConfig = Boolean(currentConfig);
  const serviceType = instanceId.split("-")[0];
  const isPlexService = serviceType === "plex";
  const requiresApiKey =
    serviceType !== "general" && serviceType !== "traefik";

  const [url, setUrl] = useState(currentConfig?.url || "");
  const [accessUrl, setAccessUrl] = useState(currentConfig?.accessUrl || "");
  const [apiKey, setApiKey] = useState(currentConfig?.apiKey || "");
  const [displayName, setDisplayName] = useState(
    currentConfig?.displayName || initialDisplayName
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { isAuthenticating, authenticate } = usePlexPinAuth();
  const [customConfig, setCustomConfig] = useState<CustomServiceConfig>({});
  // For an existing general service, the real definition hasn't loaded yet
  // (see the fetch below) - saving before it arrives would PUT the
  // still-default empty customConfig and wipe the stored definition. A
  // brand-new general service has nothing to lose, so it starts "loaded".
  const [customConfigLoaded, setCustomConfigLoaded] = useState(
    !(serviceType === "general" && hasExistingConfig)
  );
  // Set only when loading an EXISTING service's definition actually failed
  // (as opposed to "still loading"), so the UI can tell the two apart
  // instead of showing "Loading definition..." forever.
  const [customConfigLoadFailed, setCustomConfigLoadFailed] = useState(false);

  // Custom service definitions live behind their own endpoint (not the
  // generic /settings config), so fetch them separately once we know this
  // is a "general" instance. A missing definition (new instance, or the
  // general API not yet available) just leaves the form at its defaults.
  useEffect(() => {
    if (serviceType !== "general") return;
    let cancelled = false;

    (async () => {
      try {
        const existing = await getGeneralConfig(instanceId);
        if (!cancelled) {
          setCustomConfig(existing || {});
          setCustomConfigLoaded(true);
          setCustomConfigLoadFailed(false);
        }
      } catch (err) {
        console.error("Failed to load custom service config:", err);
        if (cancelled) return;
        // For a brand-new instance there's no stored definition to lose,
        // so still allow saving. For an existing one, leave
        // customConfigLoaded false - handleSubmit blocks the save rather
        // than risk overwriting the stored definition with an empty one -
        // and surface the failure so it isn't silently stuck loading.
        if (!hasExistingConfig) {
          setCustomConfigLoaded(true);
          return;
        }
        const message =
          err instanceof Error
            ? err.message
            : "Failed to load the existing custom service definition";
        setCustomConfigLoadFailed(true);
        setError(message);
        toast.error(message);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [serviceType, instanceId, hasExistingConfig]);

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
      throw new Error("Failed to validate configuration", { cause: err });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (isPlexService && !hasExistingConfig && apiKey.trim() === "") {
        throw new Error("Authenticate with Plex first");
      }

      // Validate the custom service definition up front so an invalid
      // definition never leaves the base service saved without it.
      let validatedCustomConfig: CustomServiceConfig | undefined;
      if (serviceType === "general") {
        if (!customConfigLoaded) {
          throw new Error(
            customConfigLoadFailed
              ? "The existing custom service definition failed to load - reload and try again before saving"
              : "Custom service definition is still loading - please wait and try again"
          );
        }
        const validation = validateCustomServiceConfig(customConfig);
        if (!validation.ok || !validation.config) {
          throw new Error(validation.errors.join("; "));
        }
        validatedCustomConfig = validation.config;
      }

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

      // The custom service definition and the base URL/apiKey/displayName
      // are two separate backend calls (PUT /api/general/{id}/config and
      // POST /settings/{id}), with no shared transaction across the API
      // boundary - a true atomic two-phase commit isn't possible here.
      // Ordering still matters for which half of a partial failure is
      // safer to leave in place:
      //  - Editing an EXISTING general service: the service row already
      //    exists, so PutConfig can run first. If it fails, the base
      //    fields are untouched and nothing changed. If it succeeds and
      //    the base-fields save then fails, the new definition is stored
      //    against the still-old URL/apiKey - recoverable by retrying the
      //    save, and no worse than the previous "new URL, stale
      //    definition" mismatch this ordering is meant to avoid.
      //  - A brand-new general service has no row yet: PutConfig 404s
      //    until updateConfiguration creates it, so the base fields must
      //    be saved first for a first-time save.
      if (serviceType === "general" && validatedCustomConfig && hasExistingConfig) {
        await saveGeneralConfig(instanceId, validatedCustomConfig);
      }

      // Update the configuration
      await updateConfiguration(instanceId, config);

      if (serviceType === "general" && validatedCustomConfig && !hasExistingConfig) {
        await saveGeneralConfig(instanceId, validatedCustomConfig);
      }

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
      case "whisparr":
      case "lidarr":
      case "readarr":
      case "bazarr":
      case "sabnzbd":
      case "prowlarr":
        return "API Key";
      case "overseerr":
        return "API Key";
      case "jellyfin":
      case "uptimekuma":
        return "API Key";
      case "nzbget":
        return "Control Password";
      case "traefik":
        return "Auth Token (Optional)";
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
      case "plex":
        return {
          prefix: "Authenticate via ",
          text: "Plex PIN flow guide",
          link: "https://forums.plex.tv/t/authenticating-with-plex/609370",
        };
      case "radarr":
      case "sonarr":
      case "whisparr":
      case "lidarr":
      case "readarr":
      case "bazarr":
      case "sabnzbd":
      case "nzbget":
      case "prowlarr":
        return {
          prefix: "Found in ",
          text:
            serviceType === "sabnzbd"
              ? "Config > General"
              : serviceType === "nzbget"
                ? "Settings > Security"
                : "Settings > General",
          link:
            serviceType === "sabnzbd"
              ? getSettingsUrl("/config/general/")
              : serviceType === "nzbget"
                ? getSettingsUrl("/#settings")
                : getSettingsUrl("/settings/general"),
        };
      case "jellyfin":
        return {
          prefix: "Found in ",
          text: "Dashboard > API Keys",
          link: getSettingsUrl("/web/index.html#!/apikeys.html"),
        };
      case "uptimekuma":
        return {
          prefix: "Found in ",
          text: "Settings > API Keys",
          link: getSettingsUrl("/settings/api-keys"),
        };
      case "overseerr":
        return {
          prefix: "Found in ",
          text: "Settings",
          link: getSettingsUrl("/settings/main"),
        };
      case "traefik":
        return {
          prefix: "Optional - ",
          text: "Bearer token or user:password for protected dashboard APIs",
          link: null,
        };
      case "qui":
        return {
          prefix: "Found in ",
          text: "Settings > API Key",
          link: getSettingsUrl("/settings"),
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
      case "qui":
        return "http://localhost:7476";
      case "jellyfin":
        return "http://localhost:8096";
      case "uptimekuma":
        return "http://localhost:3001";
      case "bazarr":
        return "http://localhost:6767";
      case "whisparr":
        return "http://localhost:6969";
      case "traefik":
        return "http://localhost:8080";
      case "sabnzbd":
        return "http://localhost:8080";
      case "nzbget":
        return "http://localhost:6789";
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

      {serviceType !== "general" &&
        (isPlexService ? (
          <div className="space-y-2">
            <Button
              type="button"
              variant="secondary"
              onClick={async () => {
                try {
                  await authenticate((token) => setApiKey(token));
                } catch (err) {
                  const message =
                    err instanceof Error
                      ? err.message
                      : "Failed to start Plex authentication";
                  toast.error(message);
                  setError(message);
                }
              }}
              disabled={isSubmitting || isAuthenticating}
            >
              {isAuthenticating ? "Waiting for Plex login..." : "Authenticate with Plex"}
            </Button>
            {apiKey.trim() !== "" && (
              <div className="text-xs text-emerald-400">Authenticated with Plex</div>
            )}
          </div>
        ) : (
          <FormInput
            id="apiKey"
            label={getApiKeyLabel()}
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={`Enter ${getApiKeyLabel()}`}
            helpText={apiKeyHelp}
            required={!hasExistingConfig && requiresApiKey}
            data-1p-ignore
          />
        ))}

      {serviceType === "general" && (
        <CustomServiceSection
          config={customConfig}
          onChange={setCustomConfig}
          url={url}
          apiKey={apiKey}
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
        <Button
          variant="primary"
          type="submit"
          disabled={isSubmitting || !customConfigLoaded}
        >
          {isSubmitting
            ? "Saving..."
            : customConfigLoadFailed
              ? "Definition failed to load"
              : !customConfigLoaded
                ? "Loading definition..."
                : "Save"}
        </Button>
      </div>
    </form>
  );
};
