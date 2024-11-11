/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState, useEffect, useRef } from "react";
import { toast } from "react-hot-toast";
import Toast from "../Toast";
import AnimatedModal from "../ui/AnimatedModal";
import { Button } from "../ui/Button";
import { ArrowTopRightOnSquareIcon } from "@heroicons/react/20/solid";

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

    // Sort devices: updates first, then online status, then name
    const sorted = [...filtered].sort((a, b) => {
      if (a.updateAvailable !== b.updateAvailable) {
        return b.updateAvailable ? 1 : -1;
      }
      if (a.online !== b.online) {
        return b.online ? 1 : -1;
      }
      return a.name.localeCompare(b.name);
    });

    setFilteredDevices(sorted);
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

  const getShortName = (fullName: string) => {
    return fullName.split(".")[0];
  };

  // Update the CopyIcon to remove its own hover state
  const CopyIcon = () => (
    <svg
      className="w-4 h-4 ml-1.5 text-gray-400"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
      />
    </svg>
  );

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={onClose}
      title={
        <div className="flex items-center justify-between">
          <span>Tailscale Devices</span>
          <span className="pl-4 text-sm font-normal text-gray-500 dark:text-gray-400">
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
            className="w-full px-3 py-2 pr-20 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-850 dark:border-gray-750 dark:text-white"
          />
          <div className="absolute inset-y-0 right-2 flex items-center pointer-events-none text-gray-500 dark:text-gray-400">
            <kbd className="inline-flex items-center justify-center space-x-1 rounded border border-gray-200 dark:border-gray-600 bg-gray-100 px-2 py-1 text-xs font-sans uppercase text-gray-500 dark:bg-gray-700 dark:text-gray-300">
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
                  className="p-4 rounded-lg bg-gray-50 dark:bg-gray-850 border border-gray-400 dark:border-gray-750"
                >
                  <div className="flex items-start justify-between">
                    <div className="space-y-1 flex-1">
                      <div className="flex flex-col gap-2">
                        <div className="flex items-start justify-between">
                          <div className="flex items-center gap-2 flex-1">
                            <button
                              onClick={() => copyToClipboard(device.name)}
                              className="font-medium text-gray-900 dark:text-white hover:text-blue-500 dark:hover:text-blue-400 transition-colors text-left inline-flex items-center cursor-pointer group"
                              title={`Click to copy: ${device.name}`}
                            >
                              {getShortName(device.name)}
                              <CopyIcon />
                            </button>
                          </div>
                          <span
                            className={`ml-2 shrink-0 px-2 py-0 text-sm font-medium rounded-md ${
                              device.online
                                ? "text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30 border border-green-100 dark:border-green-900/50"
                                : "text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50"
                            }`}
                          >
                            {device.online ? "Online" : "Offline"}
                          </span>
                        </div>

                        {device.tags && device.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {device.tags.map((tag, index) => (
                              <span
                                key={index}
                                className="px-1.5 py-0.5 mb-1 text-xs font-medium rounded-md bg-blue-100 text-blue-800 dark:bg-blue-750 dark:text-blue-250"
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>

                      <div className="text-sm">
                        <p className="text-gray-600 dark:text-gray-300 space-y-0.5">
                          <span className="font-medium">Type:</span>{" "}
                          {device.deviceType}
                        </p>
                        <p className="text-gray-600 dark:text-gray-300">
                          <span className="font-medium">IP:</span>{" "}
                          <button
                            onClick={() => copyToClipboard(device.ipAddress)}
                            className="hover:text-blue-500 dark:hover:text-blue-400 transition-colors inline-flex items-center cursor-pointer group"
                            title="Click to copy IP address"
                          >
                            {device.ipAddress}
                            <CopyIcon />
                          </button>
                        </p>
                        <p className="pt-2 text-gray-500 dark:text-gray-400 text-xs">
                          <span className="font-medium">Version:</span>{" "}
                          <span className="font-extrabold">
                            {trimVersion(device.clientVersion)}
                          </span>
                          {device.updateAvailable && (
                            <>
                              {" - "}
                              <a
                                href="https://tailscale.com/changelog#client"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-amber-500 dark:text-amber-400 inline-flex items-center gap-1 hover:opacity-80"
                              >
                                Update Available
                                <ArrowTopRightOnSquareIcon className="h-3 w-3" />
                              </a>
                            </>
                          )}
                        </p>
                        <p className="text-gray-500 dark:text-gray-400 text-xs">
                          Last seen: {formatLastSeen(device.lastSeen)}
                        </p>
                      </div>
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
