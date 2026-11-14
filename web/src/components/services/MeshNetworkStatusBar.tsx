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
import { useConfiguration } from "../../contexts/useConfiguration";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import { useServiceData } from "../../hooks/useServiceData";
import AnimatedModal from "../ui/AnimatedModal";
import {
  NETWORK_DESCRIPTORS,
  NETWORK_ORDER,
  memberCounts,
  findInstanceID,
  networkStatus,
  type NetworkDescriptor
} from "../../utils/meshNetwork";

const LOGOS: Record<NetworkDescriptor["type"], string> = {
  tailscale: tailscaleLogo,
  netbird: netbirdLogo,
};

interface NetworkButtonProps {
  descriptor: NetworkDescriptor;
  instanceId: string;
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

  const service = getService(instanceId);

  const items = useMemo(() => descriptor.members(service), [descriptor, service]);
  const counts = memberCounts(items, descriptor.reportedCounts(service));
  const statusKind = networkStatus({ status: service?.status, online: counts.online });
  const serverUpdate = descriptor.serverUpdateAvailable(service);
  const isDiscovered = Boolean(configurations[instanceId]?.discovered);
  const isLoading = statusKind === "loading";

  const handleConfirmRemove = async () => {
    await removeServiceInstance(instanceId);
    setIsRemoveModalOpen(false);
  };

  const label = descriptor.label;

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
      case "warning":
        return (
          <span className="text-sm flex items-center justify-center">
            {label}:<span className="text-yellow-500 ml-1">{service?.message}</span>
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
            <span className="text-green-500 ml-1">{descriptor.connectedText(counts)}</span>
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
            title={`The ${label} management server is older than the newest release`}
          >
            Update available
          </span>
        )}
        {!isDiscovered && (
          <button
            onClick={() => setIsConfigModalOpen(true)}
            className="p-1 text-gray-400 hover:text-blue-400 transition-colors"
            title={`Configure ${label}`}
            data-testid={`${descriptor.type}-configure`}
          >
            <Cog6ToothIcon className="w-4 h-4" />
          </button>
        )}
        {!isDiscovered && (
          <button
            onClick={() => setIsRemoveModalOpen(true)}
            className="p-1 text-gray-400 hover:text-blue-400 transition-colors"
            title={`Remove ${label}`}
          >
            <TrashIcon className="w-4 h-4" />
          </button>
        )}
      </div>

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

      <MeshNetworkListModal
        isOpen={isListOpen}
        onClose={() => setIsListOpen(false)}
        descriptor={descriptor}
        items={items}
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
 * MeshNetworkStatusBar shows one button per configured mesh network in the page header.
 * With only Tailscale or only NetBird set up, only that button appears; with both,
 * they stand next to each other and each opens its own list.
 */
export const MeshNetworkStatusBar: React.FC = () => {
  const { configurations } = useConfiguration();

  return (
    <>
      {NETWORK_ORDER.map((type) => {
        const descriptor = NETWORK_DESCRIPTORS[type];
        const instanceId = findInstanceID(configurations, descriptor.instancePrefix);
        return instanceId ? (
          <NetworkButton key={type} descriptor={descriptor} instanceId={instanceId} />
        ) : null;
      })}
    </>
  );
};

export default MeshNetworkStatusBar;
