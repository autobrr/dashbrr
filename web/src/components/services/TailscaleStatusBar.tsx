/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useState, useCallback, useMemo } from "react";
import { Cog6ToothIcon } from "@heroicons/react/24/solid";
import TailscaleDeviceModal from "./TailscaleDeviceModal";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useAuth } from "../../contexts/AuthContext";
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

interface ErrorResponse {
  error: string;
}

interface TailscaleStatusBarProps {
  initialConfigOpen?: boolean;
  onConfigOpen?: () => void;
}

export const TailscaleStatusBar: React.FC<TailscaleStatusBarProps> = () => {
  const [isDeviceModalOpen, setIsDeviceModalOpen] = useState(false);
  const [devices, setDevices] = useState<Device[]>([]);
  const [isOnline, setIsOnline] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { configurations } = useConfiguration();
  const { isAuthenticated, loading } = useAuth();
  const { removeServiceInstance } = useServiceManagement();
  const [isRemoveModalOpen, setIsRemoveModalOpen] = useState(false);

  const config = useMemo(() => {
    const tailscaleConfig = Object.entries(configurations).find(([id]) =>
      id.startsWith("tailscale-")
    );
    if (!tailscaleConfig) return null;
    return {
      id: tailscaleConfig[0],
      url: tailscaleConfig[1].url,
      apiKey: tailscaleConfig[1].apiKey,
    };
  }, [configurations]);

  const fetchDevices = useCallback(async () => {
    if (!config?.url || !config?.apiKey) {
      setError("Configuration missing");
      return;
    }

    if (!isAuthenticated) {
      setError("Not authenticated");
      return;
    }

    try {
      const params = new URLSearchParams({
        url: config.url,
        apiKey: config.apiKey,
      });

      const response = await api.get<DevicesResponse>(
        `/tailscale/devices?${params.toString()}`
      );

      const deviceData = response.devices || [];
      setDevices(deviceData);
      const hasOnlineDevices = deviceData.some((device) => device.online);
      setIsOnline(hasOnlineDevices);
      setError(null);
    } catch (err) {
      console.error("Failed to fetch Tailscale devices:", err);

      if (err instanceof Error) {
        if ("code" in err && err.code === "ECONNABORTED") {
          setError("Connection timeout");
        } else if ("code" in err && err.code === "ERR_NETWORK") {
          setError("Network error - Is the backend running?");
        } else {
          const error = err as {
            response?: { data?: ErrorResponse; status?: number };
          };
          if (error.response?.status === 401) {
            setError("Not authenticated");
          } else if (
            error.response?.data?.error &&
            error.response.data.error.includes("API token invalid")
          ) {
            setError("Invalid API token");
          } else if (error.response?.data?.error) {
            setError(error.response.data.error);
          } else {
            setError(err.message || "Failed to fetch devices");
          }
        }
      } else {
        setError("Unknown error occurred");
      }

      setIsOnline(false);
      setDevices([]);
    }
  }, [config?.url, config?.apiKey, isAuthenticated]);

  useEffect(() => {
    if (!loading && isAuthenticated && config?.url && config?.apiKey) {
      fetchDevices();
      const interval = setInterval(fetchDevices, 60000);
      return () => clearInterval(interval);
    } else {
      setDevices([]);
      setIsOnline(null);
      if (!isAuthenticated && !loading) {
        setError("Not authenticated");
      } else if (!config) {
        setError("Not configured");
      }
    }
  }, [
    config?.url,
    config?.apiKey,
    fetchDevices,
    isAuthenticated,
    loading,
    config,
  ]);

  const handleRemoveClick = () => {
    setIsRemoveModalOpen(true);
  };

  const handleConfirmRemove = async () => {
    if (config?.id) {
      await removeServiceInstance(config.id);
      setIsRemoveModalOpen(false);
    }
  };

  const baseButtonClasses =
    "flex items-center font-medium text-gray-300 dark:text-gray-300";

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
                : error === "Network error - Is the backend running?"
                ? "Network Error"
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

  // Only render if Tailscale is configured
  if (!config) {
    return null;
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
