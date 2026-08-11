/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Menu, Transition } from "@headlessui/react";
import { Fragment, useState, useEffect, useMemo } from "react";
import { ServiceType } from "../types/service";
import AnimatedModal from "./ui/AnimatedModal";
import { FormInput } from "./ui/FormInput";
import { api } from "../utils/api";
import { toast } from "react-hot-toast";
import { useConfiguration } from "../contexts/useConfiguration";
import { usePlexPinAuth } from "../hooks/usePlexPinAuth";
import { useTranslation } from "react-i18next";

interface AddServicesMenuProps {
  serviceTemplates: Array<{
    type: ServiceType;
    name: string;
  }>;
  onAddService: (type: ServiceType, name: string) => void;
  showServiceConfig: boolean;
  pendingService: {
    type: ServiceType;
    name: string;
    instanceId: string;
    displayName: string;
  } | null;
  onConfirmService: (
    url: string,
    apiKey: string,
    displayName: string,
    accessUrl?: string
  ) => void;

  onCancelService: () => void;
}

// Add service categories
const SERVICE_CATEGORIES = {
  AUTOMATION: "Automation Tools",
  MEDIA_SERVER: "Media Servers",
  MEDIA_MANAGEMENT: "Media Management",
  REQUESTS: "Media Requests",
  MONITORING: "Monitoring",
  NETWORK: "Network",
} as const;
type ServiceCategoryKey = keyof typeof SERVICE_CATEGORIES;

// Add category mapping
const SERVICE_CATEGORY_MAP: Record<ServiceType, ServiceCategoryKey> = {
  autobrr: "AUTOMATION",
  radarr: "MEDIA_MANAGEMENT",
  sonarr: "MEDIA_MANAGEMENT",
  lidarr: "MEDIA_MANAGEMENT",
  readarr: "MEDIA_MANAGEMENT",
  bazarr: "MEDIA_MANAGEMENT",
  sabnzbd: "MEDIA_MANAGEMENT",
  nzbget: "MEDIA_MANAGEMENT",
  prowlarr: "MEDIA_MANAGEMENT",
  traefik: "MONITORING",
  plex: "MEDIA_SERVER",
  jellyfin: "MEDIA_SERVER",
  uptimekuma: "MONITORING",
  seerr: "REQUESTS",
  maintainerr: "REQUESTS",
  qui: "AUTOMATION",
  general: "MONITORING",
  tailscale: "NETWORK",
  other: "MONITORING",
};

const CATEGORY_ORDER: ServiceCategoryKey[] = [
  "AUTOMATION",
  "MEDIA_SERVER",
  "MEDIA_MANAGEMENT",
  "REQUESTS",
  "MONITORING",
  "NETWORK",
];

type FormHelp = { prefix: string; text: string; link: string | null };
type ApiHelpContext = {
  getSettingsUrl: (path: string) => string | null;
  url: string;
};

const API_KEY_LABELS: Partial<Record<ServiceType, string>> = {
  tailscale: "API Token",
  nzbget: "Control Password",
  traefik: "Auth Token (Optional)",
};

const URL_PLACEHOLDERS: Partial<Record<ServiceType, string>> = {
  plex: "http://localhost:32400",
  jellyfin: "http://localhost:8096",
  uptimekuma: "http://localhost:3001",
  qui: "http://localhost:7476",
  traefik: "http://localhost:8080",
  bazarr: "http://localhost:6767",
  sabnzbd: "http://localhost:8080",
  nzbget: "http://localhost:6789",
  general: "Enter full URL including health endpoint",
  tailscale: "https://api.tailscale.com",
};

const API_KEY_HELP_BY_SERVICE: Partial<
  Record<ServiceType, (ctx: ApiHelpContext) => FormHelp>
> = {
  autobrr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > API",
    link: getSettingsUrl("/settings/api"),
  }),
  radarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  sonarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  lidarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  readarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  bazarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  jellyfin: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Dashboard > API Keys",
    link: getSettingsUrl("/web/index.html#!/apikeys.html"),
  }),
  uptimekuma: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > API Keys",
    link: getSettingsUrl("/settings/api-keys"),
  }),
  sabnzbd: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Config > General",
    link: getSettingsUrl("/config/general/"),
  }),
  nzbget: ({ getSettingsUrl }) => ({
    prefix: "Use ",
    text: "ControlPassword (or username:password)",
    link: getSettingsUrl("/?tab=config#S_SECURITY"),
  }),
  prowlarr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > General",
    link: getSettingsUrl("/settings/general"),
  }),
  traefik: () => ({
    prefix: "Optional - ",
    text: "Bearer token or user:password for protected dashboard APIs",
    link: null,
  }),
  seerr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings",
    link: getSettingsUrl("/settings/main"),
  }),
  maintainerr: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings",
    link: getSettingsUrl("/settings/main"),
  }),
  qui: ({ getSettingsUrl }) => ({
    prefix: "Found in ",
    text: "Settings > API Key",
    link: getSettingsUrl("/settings"),
  }),
  general: () => ({
    prefix: "Optional - ",
    text: "api token for authentication if required",
    link: null,
  }),
  tailscale: ({ url }) => ({
    prefix: "Found in ",
    text: "Admin Console > Settings > Keys",
    link:
      url === "https://api.tailscale.com"
        ? "https://login.tailscale.com/admin/settings/keys"
        : null,
  }),
};

const EMPTY_HELP: FormHelp = { prefix: "", text: "", link: null };

export function AddServicesMenu({
  serviceTemplates,
  onAddService,
  showServiceConfig,
  pendingService,
  onConfirmService,
  onCancelService,
}: AddServicesMenuProps) {
  const { t } = useTranslation();
  const [displayName, setDisplayName] = useState(
    pendingService?.displayName || ""
  );
  const [url, setUrl] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { configurations } = useConfiguration();
  const { isAuthenticating, authenticate } = usePlexPinAuth();
  const isPlexService = pendingService?.type === "plex";

  // Filter out Tailscale if it's already configured
  const availableTemplates = useMemo(() => {
    const hasTailscale = Object.keys(configurations).some((id) =>
      id.startsWith("tailscale-")
    );
    return serviceTemplates.filter(
      (template) => !(template.type === "tailscale" && hasTailscale)
    );
  }, [serviceTemplates, configurations]);

  const [accessUrl, setAccessUrl] = useState("");

  const getSettingsUrl = (path: string): string | null => {
    if (!url) return null;
    const baseUrl = accessUrl || url;
    return `${baseUrl}${path}`;
  };

  // Reset form fields when modal is opened/closed or when pending service changes
  useEffect(() => {
    if (!showServiceConfig) {
      setUrl("");
      setApiKey("");
      setDisplayName("");
      setAccessUrl("");
      setError(null);
    } else if (pendingService) {
      setDisplayName(pendingService.displayName);
      // Set default URL for Tailscale only if URL is empty
      if (pendingService.type === "tailscale") {
        setUrl("https://api.tailscale.com");
      }
    }
  }, [showServiceConfig, pendingService]);

  const validateTailscaleApiToken = async (token: string) => {
    try {
      const response = await api.get<{ status: string; error?: string }>(
        `/api/tailscale/devices?apiKey=${token}`
      );

      if (response.error) {
        throw new Error(response.error);
      }

      return true;
    } catch (err) {
      console.error("Validation error:", err);
      if (err instanceof Error) {
        throw err;
      }
      throw new Error("Failed to validate API token", { cause: err });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (isPlexService && apiKey.trim() === "") {
        throw new Error("Authenticate with Plex first");
      }

      // Special handling for Tailscale
      if (pendingService?.type === "tailscale") {
        await validateTailscaleApiToken(apiKey);
      }

      onConfirmService(
        url,
        apiKey,
        displayName || pendingService?.displayName || "",
        accessUrl || undefined
      );
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to configure service";
      toast.error(errorMessage);
      setError(errorMessage);
      console.error("Configuration error:", err);
    } finally {
      setIsSubmitting(false);
    }
  };

  const serviceType = pendingService?.type;
  const apiKeyLabel = serviceType ? API_KEY_LABELS[serviceType] || "API Key" : "API Key";
  const urlPlaceholder = serviceType
    ? URL_PLACEHOLDERS[serviceType] || "Enter service URL"
    : "Enter service URL";
  const apiKeyHelp =
    (serviceType &&
      API_KEY_HELP_BY_SERVICE[serviceType]?.({
        getSettingsUrl,
        url,
      })) ||
    EMPTY_HELP;

  const groupedServices = useMemo(() => {
    const grouped: Record<ServiceCategoryKey, typeof serviceTemplates> = {
      AUTOMATION: [],
      MEDIA_SERVER: [],
      MEDIA_MANAGEMENT: [],
      REQUESTS: [],
      MONITORING: [],
      NETWORK: [],
    };

    const normalizedSearch = searchQuery.trim().toLowerCase();
    for (const service of availableTemplates) {
      if (
        normalizedSearch &&
        !service.name.toLowerCase().includes(normalizedSearch)
      ) {
        continue;
      }
      grouped[SERVICE_CATEGORY_MAP[service.type]].push(service);
    }

    return grouped;
  }, [availableTemplates, searchQuery]);

  const isApiKeyRequired =
    pendingService?.type !== "general" && pendingService?.type !== "traefik";

  // Clear search when menu closes
  const handleMenuClose = () => {
    setSearchQuery("");
  };

  return (
    <>
      <Menu as="div" className="relative inline-block text-left z-10">
        {({ open }) => (
          <>
            <div>
              <Menu.Button className="px-2 py-2 text-sm bg-zinc-800 text-white rounded-md hover:bg-blue-600 transition-colors flex items-center gap-1">
                <span>{t("service.add_service")}</span>
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </Menu.Button>
            </div>
            <Transition
              show={open}
              as={Fragment}
              afterLeave={handleMenuClose}
              enter="transition ease-out duration-100"
              enterFrom="transform opacity-0 scale-95"
              enterTo="transform opacity-100 scale-100"
              leave="transition ease-in duration-75"
              leaveFrom="transform opacity-100 scale-100"
              leaveTo="transform opacity-0 scale-95"
            >
              <Menu.Items className="absolute right-0 w-64 mt-2 bg-white rounded-md shadow-lg border-2 border-zinc-700 dark:bg-zinc-800 focus:outline-none max-h-[calc(100vh-100px)] overflow-y-auto overflow-x-hidden origin-top-right sm:right-0 sm:left-auto left-0">
                <div className="p-2">
                  <div className="px-2 pb-2">
                    <input
                      type="text"
                      placeholder="Search services..."
                      className="w-full px-3 py-1 text-sm text-zinc-300 bg-zinc-100 dark:bg-zinc-700 rounded-md"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                    />
                  </div>

                  {CATEGORY_ORDER.map((category) => {
                    const services = groupedServices[category];
                    if (services.length === 0) return null;

                    return (
                      <div key={category} className="mb-2">
                        <div className="px-3 py-1 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                          {SERVICE_CATEGORIES[category]}
                        </div>
                        {services.map((template) => (
                          <Menu.Item key={template.type}>
                            {({ active }) => (
                              <button
                                onClick={() =>
                                  onAddService(template.type, template.name)
                                }
                                className={`block w-full px-4 py-2 text-left text-sm ${
                                  active
                                    ? "bg-zinc-100 dark:bg-zinc-700 text-zinc-900 dark:text-zinc-300"
                                    : "text-zinc-700 dark:text-zinc-300"
                                }`}
                              >
                                Add {template.name}
                              </button>
                            )}
                          </Menu.Item>
                        ))}
                      </div>
                    );
                  })}
                </div>
              </Menu.Items>
            </Transition>
          </>
        )}
      </Menu>

      <AnimatedModal
        isOpen={showServiceConfig}
        onClose={onCancelService}
        title={`Configure ${pendingService?.name || "Service"}`}
      >
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="space-y-4">
            <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
              Basic Settings
            </h3>
            <FormInput
              id="displayName"
              label="Display Name"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={pendingService?.displayName || "Enter name"}
            />
          </div>

          <div className="space-y-4">
            <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
              Connection Settings
            </h3>
            {pendingService?.type !== "tailscale" && (
              <FormInput
                id="url"
                label="URL"
                type="text"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={urlPlaceholder}
                required
                data-1p-ignore
              />
            )}

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

            {!isPlexService && (
              <FormInput
                id="apiKey"
                label={apiKeyLabel}
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={`Enter ${apiKeyLabel}`}
                helpText={apiKeyHelp}
                required={isApiKeyRequired}
                data-1p-ignore
              />
            )}
            {isPlexService && (
              <div className="mt-2">
                <button
                  type="button"
                  className="px-3 py-2 text-sm font-medium text-zinc-700 bg-white border border-zinc-300 rounded-md hover:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:bg-zinc-700 dark:text-zinc-300 dark:border-zinc-600 dark:hover:bg-zinc-600"
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
                </button>
                {apiKey.trim() !== "" && (
                  <div className="mt-2 text-xs text-emerald-400">Authenticated with Plex</div>
                )}
              </div>
            )}

            {error && (
              <div className="text-red-600 dark:text-red-400 text-sm">
                {error}
              </div>
            )}
          </div>

          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={onCancelService}
              className="px-4 py-2 text-sm font-medium text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-700 rounded-md transition-colors"
              disabled={isSubmitting}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center min-w-[100px]"
            >
              {isSubmitting ? t("service.adding", "Adding...") : t("service.add_service")}
            </button>
          </div>
        </form>
      </AnimatedModal>
    </>
  );
}
