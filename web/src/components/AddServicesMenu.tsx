/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Menu, Transition } from "@headlessui/react";
import { Fragment, useState, useEffect, useMemo } from "react";
import { ServiceType } from "../types/service";
import AnimatedModal from "../../src/components/ui/AnimatedModal";
import { FormInput } from "./ui/FormInput";
import { api } from "../utils/api";
import { toast } from "react-hot-toast";
import { useConfiguration } from "../contexts/useConfiguration";

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

// Add category mapping
const SERVICE_CATEGORY_MAP: Record<
  ServiceType,
  keyof typeof SERVICE_CATEGORIES
> = {
  autobrr: "AUTOMATION",
  omegabrr: "AUTOMATION",
  radarr: "MEDIA_MANAGEMENT",
  sonarr: "MEDIA_MANAGEMENT",
  prowlarr: "MEDIA_MANAGEMENT",
  plex: "MEDIA_SERVER",
  overseerr: "REQUESTS",
  maintainerr: "REQUESTS",
  general: "MONITORING",
  tailscale: "NETWORK",
  other: "MONITORING",
};

export function AddServicesMenu({
  serviceTemplates,
  onAddService,
  showServiceConfig,
  pendingService,
  onConfirmService,
  onCancelService,
}: AddServicesMenuProps) {
  const [displayName, setDisplayName] = useState(
    pendingService?.displayName || ""
  );
  const [url, setUrl] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { configurations } = useConfiguration();

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
      if (pendingService.type === "tailscale" && url === "") {
        setUrl("https://api.tailscale.com");
      }
    }
  }, [showServiceConfig, pendingService, url]);

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
      throw new Error("Failed to validate API token");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
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

  const getApiKeyLabel = () => {
    if (!pendingService) return "API Key";

    switch (pendingService.type) {
      case "plex":
        return "X-Plex-Token";
      case "radarr":
      case "sonarr":
      case "prowlarr":
        return "API Key";
      case "overseerr":
        return "API Key";
      case "general":
        return "API Key";
      case "tailscale":
        return "API Token";
      default:
        return "API Key";
    }
  };

  const getApiKeyHelp = () => {
    if (!pendingService) return { prefix: "", text: "", link: null };

    switch (pendingService.type) {
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
      case "maintainerr":
        return {
          prefix: "Found in ",
          text: "Settings",
          link: getSettingsUrl("/settings/main"),
        };
      case "general":
        return {
          prefix: "Optional - ",
          text: "api token for authentication if required",
          link: null,
        };
      case "tailscale":
        return {
          prefix: "Found in ",
          text: "Admin Console > Settings > Keys",
          link:
            url === "https://api.tailscale.com"
              ? "https://login.tailscale.com/admin/settings/keys"
              : null,
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
    if (!pendingService) return "Enter service URL";

    switch (pendingService.type) {
      case "plex":
        return "http://localhost:32400";
      case "general":
        return "Enter full URL including health endpoint";
      case "tailscale":
        return "https://api.tailscale.com";
      default:
        return "Enter service URL";
    }
  };

  // Group services by category
  const groupedServices = availableTemplates.reduce((acc, service) => {
    const category = SERVICE_CATEGORY_MAP[service.type] || "OTHER";
    if (!acc[category]) acc[category] = [];
    acc[category].push(service);
    return acc;
  }, {} as Record<string, typeof serviceTemplates>);

  // Filter services based on search
  const filterServices = (services: typeof serviceTemplates) => {
    if (!searchQuery) return services;
    return services.filter((s) =>
      s.name.toLowerCase().includes(searchQuery.toLowerCase())
    );
  };

  const apiKeyHelp = getApiKeyHelp();
  const isApiKeyRequired = pendingService?.type !== "general";

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
              <Menu.Button className="px-2 py-2 text-sm bg-gray-800 text-white rounded-md hover:bg-blue-800 transition-colors flex items-center gap-1">
                <span>Add Service</span>
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
              <Menu.Items className="absolute right-0 w-64 mt-2 bg-white rounded-md shadow-lg border-2 border-gray-750 dark:bg-gray-800 focus:outline-none max-h-[calc(100vh-100px)] overflow-y-auto overflow-x-hidden origin-top-right sm:right-0 sm:left-auto left-0">
                <div className="p-2">
                  <div className="px-2 pb-2">
                    <input
                      type="text"
                      placeholder="Search services..."
                      className="w-full px-3 py-1 text-sm text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                    />
                  </div>

                  {Object.entries(groupedServices).map(
                    ([category, services]) => {
                      const filteredServices = filterServices(services);
                      if (filteredServices.length === 0) return null;

                      return (
                        <div key={category} className="mb-2">
                          <div className="px-3 py-1 text-xs font-semibold text-gray-500 dark:text-gray-400">
                            {
                              SERVICE_CATEGORIES[
                                category as keyof typeof SERVICE_CATEGORIES
                              ]
                            }
                          </div>
                          {filteredServices.map((template) => (
                            <Menu.Item key={template.type}>
                              {({ active }) => (
                                <button
                                  onClick={() =>
                                    onAddService(template.type, template.name)
                                  }
                                  className={`block w-full px-4 py-2 text-left text-sm ${
                                    active
                                      ? "bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-gray-300"
                                      : "text-gray-700 dark:text-gray-300"
                                  }`}
                                >
                                  Add {template.name}
                                </button>
                              )}
                            </Menu.Item>
                          ))}
                        </div>
                      );
                    }
                  )}
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
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">
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
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Connection Settings
            </h3>
            {pendingService?.type !== "tailscale" && (
              <FormInput
                id="url"
                label="URL"
                type="text"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={getUrlPlaceholder()}
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

            <FormInput
              id="apiKey"
              label={getApiKeyLabel()}
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={`Enter ${getApiKeyLabel()}`}
              helpText={apiKeyHelp}
              required={isApiKeyRequired}
              data-1p-ignore
            />

            {error && (
              <div className="text-red-600 dark:text-red-400 text-sm">
                {error}
              </div>
            )}
          </div>

          <div className="mt-6 flex justify-end space-x-3">
            <button
              type="button"
              onClick={onCancelService}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600"
              disabled={isSubmitting}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              disabled={isSubmitting}
            >
              {isSubmitting ? "Adding..." : "Add Service"}
            </button>
          </div>
        </form>
      </AnimatedModal>
    </>
  );
}
