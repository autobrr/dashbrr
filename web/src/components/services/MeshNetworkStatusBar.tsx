/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useMemo, useState } from "react";
import { Cog6ToothIcon, TrashIcon } from "@heroicons/react/24/solid";
import tailscaleLogo from "../../assets/tailscale.svg";
import netbirdLogo from "../../assets/netbird.svg";
import { ConfigurationForm } from "../configuration/ConfigurationForm";
import MeshNetworkListModal from "./MeshNetworkListModal";
import type { NetworkListItem } from "./MeshNetworkListModal";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import { useServiceData } from "../../hooks/useServiceData";
import AnimatedModal from "../ui/AnimatedModal";
import type { NetBirdPeer, TailscaleDevice } from "../../types/service";
import {
  NETWORK_DESCRIPTORS,
  NETWORK_ORDER,
  countOnline,
  findInstanceID,
  networkStatus,
  toNetBirdMember,
  toTailscaleMember,
  type NetworkDescriptor
} from "../../utils/meshNetwork";

const LOGOS: Record<NetworkDescriptor["type"], string> = {
  tailscale: tailscaleLogo,
  netbird: netbirdLogo,
};

const TAILSCALE_UPDATE_LINK = "https://tailscale.com/changelog#client";
const NETBIRD_UPDATE_LINK = "https://github.com/netbirdio/netbird/releases";

const tailscaleItems = (devices: TailscaleDevice[]): NetworkListItem[] =>
  devices.map((device) => ({
    ...toTailscaleMember(device),
    lastSeen: device.lastSeen,
    rowFields: [
      { label: "Type", value: device.deviceType },
      { label: "IP", value: device.ipAddress, copyable: true },
      { label: "Version", value: device.clientVersion?.split("-")[0], badge: true },
      ...(device.tags && device.tags.length > 0
        ? [{ label: "Tags", value: device.tags.join(", ") }]
        : []),
    ],
  }));

const netbirdItems = (peers: NetBirdPeer[]): NetworkListItem[] =>
  peers.map((peer) => ({
    ...toNetBirdMember(peer),
    lastSeen: peer.lastSeen,
    rowFields: [
      { label: "OS", value: peer.os },
      { label: "IP", value: peer.ip, copyable: true },
      {
        label: "Groups",
        value: peer.groups && peer.groups.length > 0 ? peer.groups.join(", ") : undefined,
      },
      { label: "Version", value: peer.version?.split("-")[0], badge: true },
      {
        label: "Location",
        value: [peer.cityName, peer.countryCode].filter(Boolean).join(", ") || undefined,
      },
    ],
  }));

interface NetworkButtonProps {
  descriptor: NetworkDescriptor;
  instanceId: string | null;
}

const fadeStyle = {
  pointerEvents: "none" as const,
  userSelect: "none" as const,
  WebkitUserSelect: "none" as const,
  MozUserSelect: "none" as const,
  msUserSelect: "none" as const,
};

const NetworkButton: React.FC<NetworkButtonProps> = ({ descriptor, instanceId }) => {
  const [isListOpen, setIsListOpen] = useState(false);
  const [isRemoveModalOpen, setIsRemoveModalOpen] = useState(false);
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(false);
  const { configurations } = useConfiguration();
  const { removeServiceInstance } = useServiceManagement();
  const { getService } = useServiceData();

  const service = instanceId ? getService(instanceId) : undefined;

  const items = useMemo(() => {
    if (descriptor.type === "tailscale") {
      const devices: TailscaleDevice[] = service?.stats?.tailscale?.devices ?? [];
      return tailscaleItems(devices);
    }

    const peers: NetBirdPeer[] = service?.stats?.netbird?.peers ?? [];
    return netbirdItems(peers);
  }, [descriptor.type, service]);

  const total = items.length;
  const onlineCount = countOnline(
    items,
    descriptor.type === "tailscale"
      ? service?.details?.tailscale?.online
      : service?.details?.netbird?.online
  );
  const statusKind = networkStatus({ status: service?.status, total, online: onlineCount });
  const serverUpdate =
    descriptor.type === "netbird" && Boolean(service?.details?.netbird?.updateAvailable);
  const isLoading = statusKind === "loading";

  const handleConfirmRemove = async () => {
    if (instanceId) {
      await removeServiceInstance(instanceId);
      setIsRemoveModalOpen(false);
    }
  };

  const label = descriptor.label;
  const plural = descriptor.noun.plural;

  const renderStatus = () => {
    switch (statusKind) {
      case "loading":
        return <div className="w-16 h-4 bg-gray-700 rounded animate-pulse"></div>;
      case "error":
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:<span className="text-red-500 ml-1">Error</span>
          </span>
        );
      case "pending":
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:<span className="text-yellow-500 ml-1">Not Configured</span>
          </span>
        );
      case "empty":
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:<span className="text-yellow-500 ml-1">{`No ${plural}`}</span>
          </span>
        );
      case "offline":
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:<span className="text-yellow-500 ml-1">Offline</span>
          </span>
        );
      default:
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:
            <span className="text-green-500 ml-1">{`${onlineCount}/${total} ${plural} online`}</span>
          </span>
        );
    }
  };

  return (
    <>
      <div className="flex align-middle items-center space-x-2">
        <button
          onClick={() => setIsListOpen(true)}
          className="flex items-center font-medium text-zinc-300"
          title={service?.message || undefined}
          disabled={isLoading}
          data-testid={`${descriptor.type}-button`}
        >
          <div className="w-6 mr-1 pb-1">
            <img
              src={LOGOS[descriptor.type]}
              alt={label}
              className="w-full h-full text-gray-300"
              draggable="false"
              style={fadeStyle}
              onContextMenu={(e) => e.preventDefault()}
            />
          </div>
          {renderStatus()}
        </button>
        {serverUpdate && (
          <span
            className="px-1.5 py-0.5 text-xs font-medium rounded-md bg-amber-900/30 text-amber-400 border border-amber-900/50"
            title="The NetBird management server is older than the newest release"
          >
            Update available
          </span>
        )}
        {instanceId && (
          <button
            onClick={() => setIsConfigModalOpen(true)}
            className="p-1 text-gray-400 hover:text-blue-400 transition-colors"
            title={`Configure ${label}`}
            data-testid={`${descriptor.type}-configure`}
          >
            <Cog6ToothIcon className="w-4 h-4" />
          </button>
        )}
        {!configurations[instanceId ?? ""]?.discovered && (
          <button
            onClick={() => setIsRemoveModalOpen(true)}
            className="p-1 text-gray-400 hover:text-blue-400 transition-colors"
            title={`Remove ${label}`}
          >
            <TrashIcon className="w-4 h-4" />
          </button>
        )}
      </div>

      {instanceId && (
        <AnimatedModal
          isOpen={isConfigModalOpen}
          onClose={() => setIsConfigModalOpen(false)}
          title={`Configure ${label}`}
          maxWidth="lg"
        >
          <ConfigurationForm
            instanceId={instanceId}
            displayName={label}
            onClose={() => setIsConfigModalOpen(false)}
          />
        </AnimatedModal>
      )}

      <MeshNetworkListModal
        isOpen={isListOpen}
        onClose={() => setIsListOpen(false)}
        descriptor={descriptor}
        items={items}
        searchPlaceholder={
          descriptor.type === "tailscale"
            ? "Search devices by name, type, IP, or tags..."
            : "Search peers by name, OS, IP, group, or city..."
        }
        updateLink={
          descriptor.type === "tailscale" ? TAILSCALE_UPDATE_LINK : NETBIRD_UPDATE_LINK
        }
      />

      <AnimatedModal
        isOpen={isRemoveModalOpen}
        onClose={() => setIsRemoveModalOpen(false)}
        title={`Remove ${label}`}
        maxWidth="sm"
      >
        <div className="space-y-4">
          <p className="text-gray-700 dark:text-gray-300">
            Are you sure you want to remove {label}?
          </p>
          <div className="flex justify-end space-x-3">
            <button
              onClick={() => setIsRemoveModalOpen(false)}
              className="px-4 py-2 rounded-lg bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleConfirmRemove}
              className="px-4 py-2 rounded-lg bg-red-500 hover:bg-red-600 text-white transition-colors"
            >
              Remove
            </button>
          </div>
        </div>
      </AnimatedModal>
    </>
  );
};

/**
 * MeshNetworkStatusBar shows one button per configured mesh VPN in the page header.
 * With only Tailscale or only NetBird set up, only that button appears; with both,
 * they stand next to each other and each opens its own list.
 */
export const MeshNetworkStatusBar: React.FC = () => {
  const { configurations } = useConfiguration();

  const networks = NETWORK_ORDER.map((type) => NETWORK_DESCRIPTORS[type]).map((descriptor) => ({
    descriptor,
    instanceId: findInstanceID(configurations, descriptor.instancePrefix),
  }));

  if (networks.every((network) => network.instanceId === null)) {
    return null;
  }

  return (
    <>
      {networks.map(({ descriptor, instanceId }) =>
        instanceId ? (
          <NetworkButton key={descriptor.type} descriptor={descriptor} instanceId={instanceId} />
        ) : null
      )}
    </>
  );
};

export default MeshNetworkStatusBar;
