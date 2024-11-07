/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Menu, Transition } from "@headlessui/react";
import { Fragment, useState, useEffect } from "react";
import { ServiceType } from "../types/service";
import AnimatedModal from "../../src/components/ui/AnimatedModal";
import { FormInput } from "./ui/FormInput";

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
  onConfirmService: (url: string, apiKey: string, displayName: string) => void;
  onCancelService: () => void;
}

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

  // Reset form fields when modal is opened/closed or when pending service changes
  useEffect(() => {
    if (!showServiceConfig) {
      setUrl("");
      setApiKey("");
      setDisplayName("");
    } else if (pendingService) {
      setDisplayName(pendingService.displayName);
    }
  }, [showServiceConfig, pendingService]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onConfirmService(
      url,
      apiKey,
      displayName || pendingService?.displayName || ""
    );
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
          link: url ? `${url}/settings/api` : null,
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
          link: url ? `${url}/settings/general` : null,
        };
      case "overseerr":
        return {
          prefix: "Found in ",
          text: "Settings",
          link: url ? `${url}/settings/main` : null,
        };
      case "maintainerr":
        return {
          prefix: "Found in ",
          text: "Settings",
          link: url ? `${url}/settings/main` : null,
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
      default:
        return "Enter service URL";
    }
  };

  const apiKeyHelp = getApiKeyHelp();

  return (
    <>
      <Menu as="div" className="relative inline-block text-left z-10">
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
          as={Fragment}
          enter="transition ease-out duration-100"
          enterFrom="transform opacity-0 scale-95"
          enterTo="transform opacity-100 scale-100"
          leave="transition ease-in duration-75"
          leaveFrom="transform opacity-100 scale-100"
          leaveTo="transform opacity-0 scale-95"
        >
          <Menu.Items className="absolute right-0 w-48 mt-2 bg-white rounded-md shadow-lg border-2 border-gray-750 dark:bg-gray-800 focus:outline-none max-h-[calc(100vh-100px)] overflow-y-auto overflow-x-hidden origin-top-right sm:right-0 sm:left-auto left-0">
            <div className="py-1">
              {serviceTemplates.map((template) => (
                <Menu.Item key={template.type}>
                  {({ active }) => (
                    <button
                      onClick={() => onAddService(template.type, template.name)}
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
          </Menu.Items>
        </Transition>
      </Menu>

      <AnimatedModal
        isOpen={showServiceConfig}
        onClose={onCancelService}
        title={`Configure ${pendingService?.name || "Service"}`}
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <FormInput
            id="displayName"
            label="Display Name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={pendingService?.displayName || "Enter name"}
          />

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

          <div className="mt-6 flex justify-end space-x-3">
            <button
              type="button"
              onClick={onCancelService}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600"
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
