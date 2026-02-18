/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useMemo, useState } from "react";
import { Cog6ToothIcon } from "@heroicons/react/24/solid";
import TailscaleDeviceModal from "./TailscaleDeviceModal";
import { useConfiguration } from "../../contexts/useConfiguration";
import tailscaleLogo from "../../assets/tailscale.svg";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import AnimatedModal from "../ui/AnimatedModal";
import { useServiceData } from "../../hooks/useServiceData";
import { TailscaleDevice } from "../../types/service";

interface TailscaleStatusBarProps {
  onConfigOpen?: () => void;
}

export const TailscaleStatusBar: React.FC<TailscaleStatusBarProps> = ({
  onConfigOpen,
}) => {
  const [isDeviceModalOpen, setIsDeviceModalOpen] = useState(false);
  const { configurations } = useConfiguration();
  const { removeServiceInstance } = useServiceManagement();
  const { getService, refreshService } = useServiceData();
  const [isRemoveModalOpen, setIsRemoveModalOpen] = useState(false);

  const instanceId = useMemo(() => {
    const tailscale = Object.entries(configurations).find(([id]) =>
      id.startsWith("tailscale-")
    );
    return tailscale?.[0] ?? null;
  }, [configurations]);

  const service = instanceId ? getService(instanceId) : undefined;
  const devices: TailscaleDevice[] = service?.stats?.tailscale?.devices || [];
  const onlineCount =
    service?.details?.tailscale?.online ??
    devices.filter((device) => device.online).length;
  const isOnline = onlineCount > 0;
  const isLoading = service?.status === "loading" || service === undefined;

  useEffect(() => {
    if (!instanceId) {
      return;
    }
    void refreshService(instanceId, "all");
  }, [instanceId, refreshService]);

  const handleRemoveClick = () => {
    setIsRemoveModalOpen(true);
  };

  const handleConfirmRemove = async () => {
    if (instanceId) {
      await removeServiceInstance(instanceId);
      setIsRemoveModalOpen(false);
    }
  };

  const baseButtonClasses =
    "flex items-center font-medium text-zinc-300";

  const getStatusDisplay = () => {
    if (isLoading) {
      return <div className="w-16 h-4 bg-gray-700 rounded animate-pulse"></div>;
    }

    if (service?.status === "error") {
      return (
        <div className="text-sm flex items-center justify-center">
          Tailscale:
          <span className="text-red-500 ml-1">Error</span>
        </div>
      );
    }

    if (service?.status === "pending") {
      return (
        <div className="text-sm flex items-center justify-center">
          Tailscale:
          <span className="text-yellow-500 ml-1">Not Configured</span>
        </div>
      );
    }

    return isOnline ? (
      <div className="text-sm flex items-center justify-center">
        <>
          Tailscale:
          <span className="text-green-500 ml-1">Connected</span>
        </>
      </div>
    ) : (
      <div className="text-sm flex items-center justify-center">
        <>
          Tailscale:
          <span className="text-yellow-500 ml-1">Offline</span>
        </>
      </div>
    );
  };

  // Render a minimal "add/configure" affordance even when not configured.
  if (!instanceId) {
    return (
      <button
        onClick={onConfigOpen}
        disabled={!onConfigOpen}
        className="flex items-center gap-2 text-zinc-300 hover:text-blue-400 transition-colors disabled:opacity-50 disabled:hover:text-zinc-300"
        title="Add Tailscale"
      >
        <div className="w-6 pb-1">
          <img
            src={tailscaleLogo}
            alt="Tailscale"
            className="w-full h-full"
            draggable="false"
            style={{
              pointerEvents: "none",
              userSelect: "none",
              WebkitUserSelect: "none",
              MozUserSelect: "none",
              msUserSelect: "none",
            }}
            onContextMenu={(e) => e.preventDefault()}
          />
        </div>
        <div className="text-sm flex items-center justify-center">
          <>
            Tailscale:
            <span className="text-yellow-500 ml-1">Add</span>
          </>
        </div>
      </button>
    );
  }

  return (
    <>
      <div className="flex align-middle items-center space-x-2">
        <button
          onClick={() => setIsDeviceModalOpen(true)}
          className={`${baseButtonClasses}`}
          title={service?.message || undefined}
          disabled={isLoading}
        >
          <div className="w-6 mr-1 pb-1">
            <img
              src={tailscaleLogo}
              alt="Tailscale"
              className="w-full h-full text-gray-300"
              draggable="false"
              style={{
                pointerEvents: "none",
                userSelect: "none",
                WebkitUserSelect: "none",
                MozUserSelect: "none",
                msUserSelect: "none",
              }}
              onContextMenu={(e) => e.preventDefault()}
            />
          </div>
          {getStatusDisplay()}
        </button>
        <button
          onClick={handleRemoveClick}
          className="p-1 text-gray-400 hover:text-blue-400 transition-colors"
          title="Remove Tailscale"
        >
          <Cog6ToothIcon className="w-4 h-4" />
        </button>
      </div>

      <TailscaleDeviceModal
        isOpen={isDeviceModalOpen}
        onClose={() => setIsDeviceModalOpen(false)}
        devices={devices}
      />

      <AnimatedModal
        isOpen={isRemoveModalOpen}
        onClose={() => setIsRemoveModalOpen(false)}
        title="Remove Tailscale"
        maxWidth="sm"
      >
        <div className="space-y-4">
          <p className="text-gray-700 dark:text-gray-300">
            Are you sure you want to remove Tailscale?
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
