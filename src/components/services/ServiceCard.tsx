import React, { useState, useEffect } from "react";
import { Service } from "../../types/service";
import { ConfigurationForm } from "../configuration/ConfigurationForm";
import { ServiceHeader } from "../ui/ServiceHeader";
import { StatusIndicator } from "../ui/StatusIndicator";
import { PlexCard } from "./plex/PlexCard";
import { OmegabrrControls } from "./omegabrr/OmegabrrControls";
import { OverseerrStats } from "./overseerr/OverseerrStats";
import { AutobrrStats } from "./autobrr/AutobrrStats";
import { MaintainerrService } from "./maintainerr/MaintainerrService";
import { SonarrStats } from "./sonarr/SonarrStats";
import { RadarrStats } from "./radarr/RadarrStats";
import { ProwlarrStats } from "./prowlarr/ProwlarrStats";
import AnimatedModal from "../ui/AnimatedModal";
import { ChevronDownIcon } from "@heroicons/react/20/solid";
import { Bars3Icon } from "@heroicons/react/24/outline";

interface DragHandleProps {
  role?: string;
  tabIndex?: number;
  "aria-disabled"?: boolean;
  "aria-pressed"?: boolean;
  "aria-roledescription"?: string;
  "aria-describedby"?: string;
  onKeyDown?: (event: React.KeyboardEvent) => void;
  onPointerDown?: (event: React.PointerEvent) => void;
}

interface ServiceCardProps {
  service: Service;
  onRemove: () => void;
  isInitialLoad?: boolean;
  isConnected?: boolean;
  dragHandleProps?: DragHandleProps;
  isDragging?: boolean;
}

const getStorageKey = (instanceId: string) =>
  `dashbrr-service-${instanceId}-collapsed`;

export const ServiceCard: React.FC<ServiceCardProps> = ({
  service,
  onRemove,
  isInitialLoad = false,
  isConnected = true,
  dragHandleProps = {},
  isDragging = false,
}) => {
  const [showConfig, setShowConfig] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(() => {
    try {
      const stored = window.localStorage.getItem(
        getStorageKey(service.instanceId)
      );
      return stored ? JSON.parse(stored) : false;
    } catch (error) {
      console.error("Error reading collapse state:", error);
      return false;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(
        getStorageKey(service.instanceId),
        JSON.stringify(isCollapsed)
      );
    } catch (error) {
      console.error("Error saving collapse state:", error);
    }
  }, [isCollapsed, service.instanceId]);

  const needsConfiguration = !service.url;
  const displayMessage = service.message;

  const renderServiceSpecificControls = () => {
    if (needsConfiguration) return null;

    switch (service.type) {
      case "autobrr":
        return (
          <div className="bg-transparent">
            <AutobrrStats instanceId={service.instanceId} />
          </div>
        );
      case "omegabrr":
        return <OmegabrrControls url={service.url!} apiKey={service.apiKey!} />;
      case "overseerr":
        return <OverseerrStats instanceId={service.instanceId} />;
      case "plex":
        return <PlexCard instanceId={service.instanceId} />;
      case "maintainerr":
        return service.url ? (
          <div className="bg-transparent">
            <MaintainerrService instanceId={service.instanceId} />
          </div>
        ) : null;
      case "sonarr":
        return <SonarrStats instanceId={service.instanceId} />;
      case "radarr":
        return <RadarrStats instanceId={service.instanceId} />;
      case "prowlarr":
        return <ProwlarrStats instanceId={service.instanceId} />;
      default:
        return null;
    }
  };

  return (
    <>
      <div
        className={`group relative bg-white dark:bg-gray-800 rounded-lg shadow-lg transition-all duration-200 ease-in-out ${
          !isDragging && "hover:scale-[1.01]"
        } ${
          needsConfiguration
            ? "border-2 border-dashed dark:border-gray-600"
            : "border border-gray-200 dark:border-gray-700"
        }`}
      >
        <div className="p-4">
          <div
            className="relative cursor-pointer select-none transition-colors -mx-4 px-4 py-0 rounded-t-lg text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
            onClick={() => setIsCollapsed(!isCollapsed)}
          >
            <div className="absolute right-4 top-1/2 -translate-y-1/2 transition-transform duration-200">
              <div
                {...dragHandleProps}
                className="opacity-30 text-gray-500 group-hover:opacity-60 transition-opacity duration-200 cursor-grab active:cursor-grabbing"
              >
                <Bars3Icon className="h-5 w-5 rotate-90" />
              </div>
            </div>
            <div className="pr-8">
              <ServiceHeader
                displayName={service.displayName}
                url={service.url}
                version={service.version}
                updateAvailable={service.updateAvailable}
                healthEndpoint={service.healthEndpoint}
                onConfigure={(e?: React.MouseEvent) => {
                  e?.stopPropagation();
                  setShowConfig(true);
                }}
                onRemove={(e?: React.MouseEvent) => {
                  e?.stopPropagation();
                  onRemove();
                }}
                needsConfiguration={needsConfiguration}
                status={
                  service.status as
                    | "online"
                    | "offline"
                    | "warning"
                    | "error"
                    | "loading"
                    | "unknown"
                }
              />
            </div>
          </div>

          <div
            className={`transition-all duration-300 ease-in-out overflow-hidden ${
              isCollapsed ? "max-h-0 opacity-0" : "max-h-[2000px] opacity-100"
            }`}
          >
            {needsConfiguration ? (
              <div className="flex items-center justify-center p-6 text-center">
                <p className="text-sm text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20 px-4 py-2 rounded-lg">
                  Click the gear icon to configure this service
                </p>
              </div>
            ) : (
              <div className="mt-4">
                {(displayMessage || !isConnected || isInitialLoad) && (
                  <StatusIndicator
                    status={
                      service.status as
                        | "online"
                        | "offline"
                        | "warning"
                        | "error"
                        | "loading"
                        | "unknown"
                    }
                    message={displayMessage}
                    isInitialLoad={isInitialLoad}
                    isConnected={isConnected}
                  />
                )}

                {renderServiceSpecificControls()}
              </div>
            )}
          </div>

          {/* Response time and Last checked */}
          <div className="mt-4 space-y-1 pointer-events-none border-gray-100 dark:border-gray-700 pt-4 select-none">
            {service.responseTime !== undefined && (
              <p className="text-xs font-medium text-gray-600 dark:text-gray-400">
                Response time:{" "}
                <span className="font-normal">{service.responseTime}ms</span>
              </p>
            )}
            {service.lastChecked && (
              <p className="text-xs font-medium text-gray-600 dark:text-gray-400">
                Last checked:{" "}
                <span className="font-normal">
                  {(() => {
                    const date = new Date(service.lastChecked);
                    const today = new Date();

                    if (date.toDateString() === today.toDateString()) {
                      return date.toLocaleTimeString();
                    } else {
                      return date.toLocaleString();
                    }
                  })()}
                </span>
              </p>
            )}
          </div>

          {/* Collapse/Expand Icon */}
          <div
            onClick={(e) => {
              e.stopPropagation();
              setIsCollapsed(!isCollapsed);
            }}
            className="absolute bottom-4 right-4 opacity-30 text-gray-300 group-hover:opacity-60 transition-opacity duration-200 cursor-pointer"
          >
            <div
              className={`transform transition-transform duration-200 ${
                isCollapsed ? "rotate-0" : "rotate-180"
              }`}
            >
              <ChevronDownIcon className="h-5 w-5" />
            </div>
          </div>
        </div>
      </div>

      <AnimatedModal
        isOpen={showConfig}
        onClose={() => setShowConfig(false)}
        title={`Configure ${service.displayName}`}
      >
        <ConfigurationForm
          serviceName={service.name}
          instanceId={service.instanceId}
          displayName={service.displayName}
          onClose={() => setShowConfig(false)}
        />
      </AnimatedModal>
    </>
  );
};
