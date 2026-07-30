/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import {
  FaDesktop,
  FaFilm,
  FaMobile,
  FaMusic,
  FaPause,
  FaPlay,
  FaPlayCircle,
  FaTablet,
  FaTv
} from "react-icons/fa";

export const formatDurationMs = (durationMs: number): string => {
  if (!Number.isFinite(durationMs) || durationMs <= 0) {
    return "0:00";
  }

  const hours = Math.floor(durationMs / 3600000);
  const minutes = Math.floor((durationMs % 3600000) / 60000);
  const seconds = Math.floor((durationMs % 60000) / 1000);

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, "0")}:${seconds
      .toString()
      .padStart(2, "0")}`;
  }
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
};

export const formatDurationTicks = (ticks?: number): string => {
  if (!ticks || ticks <= 0) {
    return "0:00";
  }
  // Jellyfin ticks are 100ns.
  const durationMs = Math.floor(ticks / 10000);
  return formatDurationMs(durationMs);
};

export const getMediaTypeIcon = (
  type: string | undefined,
  isPaused: boolean
): React.ReactNode => {
  if (isPaused) {
    return <FaPause className="text-gray-500 dark:text-gray-400 h-4 w-4" />;
  }

  switch ((type || "").toLowerCase()) {
    case "track":
    case "song":
      return <FaMusic className="text-blue-600 dark:text-blue-400 h-4 w-4" />;
    case "movie":
      return <FaFilm className="text-amber-500 dark:text-amber-300 h-4 w-4" />;
    case "episode":
    case "show":
      return <FaTv className="text-green-600 dark:text-green-400 h-4 w-4" />;
    case "clip":
    case "trailer":
      return (
        <FaPlayCircle className="text-purple-500 dark:text-purple-400 h-4 w-4" />
      );
    default:
      return <FaPlay className="text-gray-500 h-4 w-4" />;
  }
};

export const getDeviceIcon = (
  platformOrDeviceType: string | undefined
): React.ReactNode => {
  const normalized = (platformOrDeviceType || "").toLowerCase();

  switch (normalized) {
    case "windows":
    case "macos":
    case "linux":
      return <FaDesktop className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    case "ios":
    case "android":
    case "iphone":
    case "ipad":
      return <FaMobile className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    case "tvos":
    case "roku":
    case "androidtv":
    case "firetv":
    case "tv":
      return <FaTv className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    default:
      return <FaTablet className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
  }
};

export const getProgressPercentage = (
  current: number,
  total: number
): number => {
  if (!Number.isFinite(current) || !Number.isFinite(total) || total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(100, Math.round((current / total) * 100)));
};

export const formatBitrateKbps = (bitrateKbps: number): string => {
  if (!Number.isFinite(bitrateKbps) || bitrateKbps <= 0) {
    return "0 Kbps";
  }
  if (bitrateKbps >= 1000) {
    return `${(bitrateKbps / 1000).toFixed(1)} Mbps`;
  }
  return `${Math.round(bitrateKbps)} Kbps`;
};

export const formatBitrateBps = (bitrateBps: number): string => {
  if (!Number.isFinite(bitrateBps) || bitrateBps <= 0) {
    return "0 bps";
  }
  if (bitrateBps >= 1_000_000) {
    return `${(bitrateBps / 1_000_000).toFixed(1)} Mbps`;
  }
  if (bitrateBps >= 1000) {
    return `${Math.round(bitrateBps / 1000)} Kbps`;
  }
  return `${Math.round(bitrateBps)} bps`;
};
