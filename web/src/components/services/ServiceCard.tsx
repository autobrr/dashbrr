/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState } from "react";
import { Service, ServiceType } from "../../types/service";
import { ConfigurationForm } from "../configuration/ConfigurationForm";
import { ServiceHeader } from "../ui/ServiceHeader";
import { PlexStats } from "./plex/PlexStats";
import { JellyfinStats } from "./jellyfin/JellyfinStats";
import { UptimeKumaStats } from "./uptimekuma/UptimeKumaStats";
import { OverseerrStats } from "./overseerr/OverseerrStats";
import { AutobrrStats } from "./autobrr/AutobrrStats";
import { MaintainerrService } from "./maintainerr/MaintainerrService";
import { BazarrStats } from "./bazarr/BazarrStats";
import { SabnzbdStats } from "./sabnzbd/SabnzbdStats";
import { NzbgetStats } from "./nzbget/NzbgetStats";
import { ProwlarrStats } from "./prowlarr/ProwlarrStats";
import { TraefikStats } from "./traefik/TraefikStats";
import { QuiStats } from "./qui/QuiStats";
import { GeneralStats } from "./general/GeneralStats";
import { ArrQueueStats } from "./common/ArrQueueStats";
import AnimatedModal from "../ui/AnimatedModal";
import { ChevronDownIcon } from "@heroicons/react/20/solid";
import { Bars3Icon } from "@heroicons/react/24/outline";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useCollapsiblePreference } from "../../hooks/useCollapsiblePreference";
import { serviceCardCollapseKey } from "../../utils/collapsePreferences";
import {
  getServiceCardLayoutClasses,
  hasMeaningfulServiceContent
} from "../../utils/serviceCardContent";

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
  dragHandleProps?: DragHandleProps;
  isDragging?: boolean;
  isConnected?: boolean;
  isInitialLoad?: boolean;
}

const formatLastChecked = (lastChecked: Date) => {
  const date = new Date(lastChecked);
  const today = new Date();
  return date.toDateString() === today.toDateString()
    ? date.toLocaleTimeString()
    : date.toLocaleString();
};

type ServiceStatsRenderer = (
  instanceId: string,
  url: string
) => React.ReactNode;

const SERVICE_STATS_RENDERERS: Partial<
  Record<ServiceType, ServiceStatsRenderer>
> = {
  autobrr: (instanceId) => (
    <div className="bg-transparent">
      <AutobrrStats instanceId={instanceId} />
    </div>
  ),
  overseerr: (instanceId) => <OverseerrStats instanceId={instanceId} />,
  plex: (instanceId) => <PlexStats instanceId={instanceId} />,
  jellyfin: (instanceId) => <JellyfinStats instanceId={instanceId} />,
  uptimekuma: (instanceId) => <UptimeKumaStats instanceId={instanceId} />,
  maintainerr: (instanceId, url) =>
    url ? (
      <div className="bg-transparent">
        <MaintainerrService instanceId={instanceId} />
      </div>
    ) : null,
  whisparr: (instanceId) => (
    <ArrQueueStats instanceId={instanceId} serviceType="whisparr" />
  ),
  sonarr: (instanceId) => (
    <ArrQueueStats instanceId={instanceId} serviceType="sonarr" />
  ),
  radarr: (instanceId) => (
    <ArrQueueStats instanceId={instanceId} serviceType="radarr" />
  ),
  lidarr: (instanceId) => (
    <ArrQueueStats instanceId={instanceId} serviceType="lidarr" />
  ),
  readarr: (instanceId) => (
    <ArrQueueStats instanceId={instanceId} serviceType="readarr" />
  ),
  bazarr: (instanceId) => <BazarrStats instanceId={instanceId} />,
  sabnzbd: (instanceId) => <SabnzbdStats instanceId={instanceId} />,
  nzbget: (instanceId) => <NzbgetStats instanceId={instanceId} />,
  prowlarr: (instanceId) => <ProwlarrStats instanceId={instanceId} />,
  traefik: (instanceId) => <TraefikStats instanceId={instanceId} />,
  qui: (instanceId) => <QuiStats instanceId={instanceId} />,
  general: (instanceId) => <GeneralStats instanceId={instanceId} />,
};

export const ServiceCard: React.FC<ServiceCardProps> = ({
  service,
  onRemove,
  dragHandleProps = {},
  isDragging = false,
  isConnected = true,
  isInitialLoad,
}) => {
  const [showConfig, setShowConfig] = useState(false);
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceCardCollapseKey(service.instanceId),
    true
  );
  const isCollapsed = !isExpanded;

  const { configurations } = useConfiguration();
  const currentConfig = configurations[service.instanceId];

  const needsConfiguration = !service.url;
  const shouldCompactContentLayout =
    !needsConfiguration &&
    isConnected &&
    !isInitialLoad &&
    !hasMeaningfulServiceContent(service);
  const layoutClasses = getServiceCardLayoutClasses(
    shouldCompactContentLayout ? "compact" : "regular"
  );

  const renderServiceSpecificControls = () => {
    if (needsConfiguration) return null;

    // Don't render controls if we're not connected or still loading
    if (!isConnected || isInitialLoad) {
      return (
        <div className="flex items-center justify-center p-6 text-center">
          <p className="text-sm text-zinc-600 dark:text-zinc-400 bg-zinc-50 dark:bg-zinc-900/20 px-4 py-2 rounded-lg">
            {isInitialLoad ? "Loading..." : "Disconnected from backend"}
          </p>
        </div>
      );
    }

    const renderer = SERVICE_STATS_RENDERERS[service.type];
    return renderer ? renderer(service.instanceId, service.url) : null;
  };
  const serviceSpecificControls = renderServiceSpecificControls();

  return (
    <>
      <div
        className={`@container group relative bg-white dark:bg-zinc-800 rounded-lg shadow-lg transition-all duration-200 ease-in-out motion-reduce:transition-none ${
          !isDragging && "motion-safe:hover:scale-[1.01]"
        } ${
          needsConfiguration
            ? "border-2 border-dashed dark:border-zinc-600"
            : "border border-zinc-200 dark:border-zinc-700"
        } ${!isConnected && "opacity-75"}`}
      >
        <div className="p-3 @md:p-4">
          <div
            className="relative cursor-pointer select-none transition-colors -mx-3 px-3 @md:-mx-4 @md:px-4 py-0 rounded-t-lg text-zinc-600 dark:text-zinc-300 hover:text-zinc-900 dark:hover:text-white"
            onClick={toggle}
          >
            <div className="absolute right-4 top-1/2 -translate-y-1/2 transition-transform duration-200">
              <div
                {...dragHandleProps}
                className="opacity-30 text-zinc-500 group-hover:opacity-60 transition-opacity duration-200 cursor-grab active:cursor-grabbing"
              >
                <Bars3Icon className="h-5 w-5 rotate-90" />
              </div>
            </div>
            <div className="pr-8 @md:pr-10">
              <ServiceHeader
                displayName={currentConfig?.displayName || service.displayName}
                url={currentConfig?.url || service.url}
                accessUrl={currentConfig?.accessUrl || service.accessUrl}
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
                status={service.status}
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
              serviceSpecificControls && (
                <div className={layoutClasses.bodyMarginClass}>
                  {serviceSpecificControls}
                </div>
              )
            )}
          </div>

          {/* Response time and Last checked */}
          <div
            className={`space-y-1 pointer-events-none border-zinc-100 dark:border-zinc-700 select-none ${layoutClasses.footerSpacingClass}`}
          >
            {service.responseTime !== undefined && (
              <p className="text-xs font-medium text-zinc-600 dark:text-zinc-400">
                Response time:{" "}
                <span className="font-normal">{service.responseTime}ms</span>
              </p>
            )}
            {service.lastChecked && (
              <p className="text-xs font-medium text-zinc-600 dark:text-zinc-400">
                Last checked:{" "}
                <span className="font-normal">{formatLastChecked(service.lastChecked)}</span>
              </p>
            )}
          </div>

          {/* Collapse/Expand Icon */}
          <div
            onClick={(e) => {
              e.stopPropagation();
              toggle();
            }}
            className="absolute bottom-4 right-4 opacity-30 text-zinc-300 group-hover:opacity-60 transition-opacity duration-200 cursor-pointer"
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
          instanceId={service.instanceId}
          displayName={service.displayName}
          onClose={() => setShowConfig(false)}
        />
      </AnimatedModal>
    </>
  );
};
