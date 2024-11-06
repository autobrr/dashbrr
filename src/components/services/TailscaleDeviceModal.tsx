/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState, useEffect, useRef } from "react";
import { toast } from "react-hot-toast";
import Toast from "../Toast";
import AnimatedModal from "../ui/AnimatedModal";
import { Button } from "../ui/Button";

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

interface Props {
  isOpen: boolean;
  onClose: () => void;
  devices: Device[];
}

const TailscaleDeviceModal: React.FC<Props> = ({
  isOpen,
  onClose,
  devices,
}) => {
  const [searchTerm, setSearchTerm] = useState("");
  const [filteredDevices, setFilteredDevices] = useState<Device[]>(devices);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Check for CMD+K (Mac) or CTRL+K (Windows/Linux)
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    };

    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [isOpen]);

  useEffect(() => {
    const filtered = devices.filter((device) => {
      const searchLower = searchTerm.toLowerCase();
      return (
        device.name?.toLowerCase().includes(searchLower) ||
        device.deviceType?.toLowerCase().includes(searchLower) ||
        device.ipAddress?.toLowerCase().includes(searchLower) ||
        device.tags?.some((tag) => tag.toLowerCase().includes(searchLower))
      );
    });
    setFilteredDevices(filtered);
  }, [searchTerm, devices]);

  const formatLastSeen = (lastSeen: string) => {
    const date = new Date(lastSeen);
    return date.toLocaleString();
  };

  const trimVersion = (version: string) => {
    return version ? version.split("-")[0] : "Unknown";
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.custom((t) => (
      <Toast type="success" body={`Copied ${text} to clipboard`} t={t} />
    ));
  };

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={onClose}
      title={
        <div className="flex items-center justify-between">
          <span>Tailscale Devices</span>
          <span className="text-sm font-normal text-gray-500 dark:text-gray-400">
            {filteredDevices.filter((d) => d.online).length} of{" "}
            {filteredDevices.length} online
          </span>
        </div>
      }
      maxWidth="2xl"
    >
      <div className="flex flex-col">
        <div className="relative mt-4 mb-4">
          <input
            ref={searchInputRef}
            type="text"
            placeholder="Search devices by name, type, IP, or tags..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full px-3 py-2 pr-20 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:border-gray-750 dark:text-white"
          />
          <div className="absolute inset-y-0 right-3 flex items-center pointer-events-none text-gray-500 dark:text-gray-400">
            <kbd className="inline-flex items-center justify-center space-x-1 rounded border bg-gray-100 px-2 py-1 text-xs font-sans uppercase text-gray-500 dark:bg-gray-700 dark:text-gray-300">
              {navigator.userAgent?.includes("Mac") ? "⌘" : "Ctrl"}+K
            </kbd>
          </div>
        </div>

        <div className="flex-1 -mr-[18px]">
          <div className="max-h-[60vh] overflow-y-auto scrollbar-small pr-[6px]">
            <div className="space-y-3">
              {filteredDevices.map((device) => (
                <div
                  key={device.id}
                  className="p-4 rounded-lg bg-gray-50 dark:bg-gray-800 border border-gray-400 dark:border-gray-750"
                >
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => copyToClipboard(device.name)}
                          className="font-medium text-gray-900 dark:text-white hover:text-blue-500 dark:hover:text-blue-400 transition-colors"
                          title="Click to copy DNS name"
                        >
                          {device.name}
                        </button>
                        {device.updateAvailable && (
                          <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
                            Update Available
                          </span>
                        )}
                      </div>
                      {device.tags && device.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-1">
                          {device.tags.map((tag, index) => (
                            <span
                              key={index}
                              className="px-2 py-0.5 text-xs font-medium rounded-full bg-blue-100 text-blue-800 dark:bg-blue-750 dark:text-blue-250"
                            >
                              {tag}
                            </span>
                          ))}
                        </div>
                      )}
                      <div className="text-sm">
                        <p className="text-gray-600 dark:text-gray-300 space-y-0.5">
                          <span className="font-medium">Type:</span>{" "}
                          {device.deviceType}
                        </p>
                        <p className="text-gray-600 dark:text-gray-300">
                          <span className="font-medium">IP:</span>{" "}
                          <button
                            onClick={() => copyToClipboard(device.ipAddress)}
                            className="hover:text-blue-500 dark:hover:text-blue-400 transition-colors"
                            title="Click to copy IP address"
                          >
                            {device.ipAddress}
                          </button>
                        </p>
                        <p className="pt-2 text-gray-500 dark:text-gray-400 text-xs">
                          <span className="font-medium">Version:</span>{" "}
                          {trimVersion(device.clientVersion)}
                        </p>
                        <p className="text-gray-500 dark:text-gray-400 text-xs">
                          Last seen: {formatLastSeen(device.lastSeen)}
                        </p>
                      </div>
                    </div>
                    <div className="flex flex-col items-end">
                      <span
                        className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                          device.online
                            ? "bg-green-100 text-green-800 dark:bg-green-700 dark:text-green-300"
                            : "bg-red-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200"
                        }`}
                      >
                        {device.online ? "Online" : "Offline"}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="mt-6 flex justify-end">
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </div>
    </AnimatedModal>
  );
};

export default TailscaleDeviceModal;
