/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Menu, Transition } from "@headlessui/react";
import { Fragment, useState, useMemo } from "react";
import { ServiceConfigInput, ServiceType } from "../types/service";
import AnimatedModal from "./ui/AnimatedModal";
import { ServiceSetupForm } from "./configuration/ServiceSetupForm";
import { useConfiguration } from "../contexts/useConfiguration";
import { ServiceSetup } from "../config/serviceSetup";
import { isMeshNetworkType } from "../utils/meshNetwork";

interface AddServicesMenuProps {
  serviceTemplates: Array<{
    type: ServiceType;
    name: string;
  }>;
  onAddService: (type: ServiceType, name: string) => void;
  showServiceConfig: boolean;
  pendingService: {
    name: string;
    setup: ServiceSetup;
  } | null;
  onConfirmService: (input: ServiceConfigInput) => Promise<void>;
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
  netbird: "NETWORK",
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
  const [searchQuery, setSearchQuery] = useState("");
  const { configurations } = useConfiguration();

  // Tailscale and NetBird show in the page header and support one instance, so
  // their entry leaves the menu once the network is configured.
  const availableTemplates = useMemo(() => {
    const configured = (type: ServiceType) =>
      Object.keys(configurations).some((id) => id.startsWith(`${type}-`));

    return serviceTemplates.filter(
      (template) =>
        !(isMeshNetworkType(template.type) && configured(template.type))
    );
  }, [serviceTemplates, configurations]);

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
        {pendingService && (
          <ServiceSetupForm
            setup={pendingService.setup}
            submitLabel="Add Service"
            onSubmit={onConfirmService}
            onCancel={onCancelService}
          />
        )}
      </AnimatedModal>
    </>
  );
}
