/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useState, useCallback, useMemo } from "react";
import { CogIcon } from "@heroicons/react/24/solid";
import TailscaleDeviceModal from "./TailscaleDeviceModal";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useAuth } from "../../contexts/AuthContext";
import { api } from "../../utils/api";
import tailscaleLogo from "../../assets/tailscale.svg";

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

export const TailscaleStatusBar: React.FC<TailscaleStatusBarProps> = ({
  onConfigOpen,
}) => {
  const [isDeviceModalOpen, setIsDeviceModalOpen] = useState(false);
  const [devices, setDevices] = useState<Device[]>([]);
  const [isOnline, setIsOnline] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { configurations } = useConfiguration();
  const { isAuthenticated, loading } = useAuth();

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

  const handleConfigClick = () => {
    onConfigOpen?.();
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

  return (
    <>
      <div className="flex align-middle items-center space-x-2">
        {config ? (
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
        ) : (
          <button
            onClick={handleConfigClick}
            className="px-3 py-1 text-sm rounded-md font-medium text-white bg-gray-800 hover:bg-gray-700"
            disabled={!isAuthenticated || loading}
          >
            <CogIcon className="w-5 h-5" />
          </button>
        )}
      </div>

      <TailscaleDeviceModal
        isOpen={isDeviceModalOpen}
        onClose={() => setIsDeviceModalOpen(false)}
        devices={devices}
      />
    </>
  );
};
