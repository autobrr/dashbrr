/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Menu, Transition } from "@headlessui/react";
import { Fragment, useState, useEffect, useMemo } from "react";
import { ServiceType } from "../types/service";
import AnimatedModal from "./ui/AnimatedModal";
import { FormInput } from "./ui/FormInput";
import { toast } from "react-hot-toast";
import { useConfiguration } from "../contexts/useConfiguration";
import { usePlexPinAuth } from "../hooks/usePlexPinAuth";
import { serviceForms } from "../config/serviceForms";

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
  whisparr: "MEDIA_MANAGEMENT",
  lidarr: "MEDIA_MANAGEMENT",
  readarr: "MEDIA_MANAGEMENT",
  bazarr: "MEDIA_MANAGEMENT",
  sabnzbd: "MEDIA_MANAGEMENT",
  nzbget: "MEDIA_MANAGEMENT",
  prowlarr: "MEDIA_MANAGEMENT",
  traefik: "MONITORING",
  plex: "MEDIA_SERVER",
  jellyfin: "MEDIA_SERVER",
  emby: "MEDIA_SERVER",
  uptimekuma: "MONITORING",
  overseerr: "REQUESTS",
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      if (isPlexService && apiKey.trim() === "") {
        throw new Error("Authenticate with Plex first");
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
    }
  };

  const form = serviceForms[pendingService?.type ?? "other"];
  const apiKeyLabel = form.apiKeyLabel;
  const urlPlaceholder = form.urlPlaceholder;
  const apiKeyHelp = form.apiKeyHelp({ url, accessUrl });

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
                  disabled={isAuthenticating}
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

          <div className="mt-6 flex justify-end space-x-3">
            <button
              type="button"
              onClick={onCancelService}
              className="px-4 py-2 text-sm font-medium text-zinc-700 bg-white border border-zinc-300 rounded-md hover:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:bg-zinc-700 dark:text-zinc-300 dark:border-zinc-600 dark:hover:bg-zinc-600"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              Add Service
            </button>
          </div>
        </form>
      </AnimatedModal>
    </>
  );
}
