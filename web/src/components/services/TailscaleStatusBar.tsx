/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useState, useCallback, useMemo } from "react";
import { Cog6ToothIcon } from "@heroicons/react/24/solid";
import TailscaleDeviceModal from "./TailscaleDeviceModal";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useAuth } from "../../hooks/useAuth";
import { api } from "../../utils/api";
import tailscaleLogo from "../../assets/tailscale.svg";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import AnimatedModal from "../ui/AnimatedModal";

interface Device {
  name: string;
  id: string;
  ipAddress: string;
  lastSeen: string;
  online: boolean;
  deviceType: string;
  clientVersion: string;
  updateAvailable: boolean;
  tags?: string[];
}

interface DevicesResponse {
  devices: Device[];
}

interface TailscaleStatusBarProps {
  onConfigOpen?: () => void;
}

export const TailscaleStatusBar: React.FC<TailscaleStatusBarProps> = ({
  onConfigOpen,
}) => {
  const [isDeviceModalOpen, setIsDeviceModalOpen] = useState(false);
  const [devices, setDevices] = useState<Device[]>([]);
  const [isOnline, setIsOnline] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { configurations } = useConfiguration();
  const { isAuthenticated, loading } = useAuth();
  const { removeServiceInstance } = useServiceManagement();
  const [isRemoveModalOpen, setIsRemoveModalOpen] = useState(false);

  const instanceId = useMemo(() => {
    const tailscale = Object.entries(configurations).find(([id]) =>
      id.startsWith("tailscale-")
    );
    return tailscale?.[0] ?? null;
  }, [configurations]);

  const formatError = useCallback((err: unknown): string => {
    const msg = err instanceof Error ? err.message : String(err);
    const lower = msg.toLowerCase();

    if (lower.includes("api token invalid")) return "Invalid API token";
    if (lower.includes("timed out")) return "Connection timeout";
    if (lower.includes("failed to fetch")) return "Network error";

    return msg || "Failed to fetch devices";
  }, []);

  const fetchDevices = useCallback(async () => {
    if (!instanceId) {
      setError("Configuration missing");
      return;
    }

    if (!isAuthenticated) {
      setError("Not authenticated");
      return;
    }

    try {
      const response = await api.get<DevicesResponse>(
        `/tailscale/devices?instanceId=${encodeURIComponent(instanceId)}`
      );

      const deviceData = response.devices || [];
      setDevices(deviceData);
      const hasOnlineDevices = deviceData.some((device) => device.online);
      setIsOnline(hasOnlineDevices);
      setError(null);
    } catch (err) {
      console.error("Failed to fetch Tailscale devices:", err);
      setError(formatError(err));

      setIsOnline(false);
      setDevices([]);
    }
  }, [formatError, instanceId, isAuthenticated]);

  useEffect(() => {
    if (!loading && isAuthenticated && instanceId) {
      fetchDevices();
      const interval = setInterval(fetchDevices, 60000);
      return () => clearInterval(interval);
    } else {
      setDevices([]);
      setIsOnline(null);
      if (!isAuthenticated && !loading) {
        setError("Not authenticated");
      } else if (!instanceId) {
        setError("Not configured");
      }
    }
  }, [instanceId, fetchDevices, isAuthenticated, loading]);

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
    if (loading || isOnline === null) {
      return <div className="w-16 h-4 bg-gray-700 rounded animate-pulse"></div>;
    }

    if (error) {
      return (
        <div className="text-sm flex items-center justify-center">
          <>
            Tailscale:
            <span className="text-red-500 ml-1">
              {error === "Invalid API token"
                ? "Invalid Token"
                : error === "Not configured"
                ? "Not Configured"
                : error === "Not authenticated"
                ? "Not Authenticated"
                : error === "Connection timeout"
                ? "Timeout"
                : error === "Network error"
                ? "Network"
                : "Error"}
            </span>
          </>
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
          title={error || undefined}
          disabled={!isAuthenticated || loading}
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
