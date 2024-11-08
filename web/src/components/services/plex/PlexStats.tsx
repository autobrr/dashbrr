/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState, useEffect, useMemo } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { PlexSession } from "../../../types/service";
import { PlexMessage } from "./PlexMessage";
import {
  FaUser,
  FaPlay,
  FaMusic,
  FaFilm,
  FaTv,
  FaDesktop,
  FaMobile,
  FaTablet,
} from "react-icons/fa";

interface PlexStatsProps {
  instanceId: string;
}

const formatDuration = (duration: number): string => {
  const hours = Math.floor(duration / 3600000);
  const minutes = Math.floor((duration % 3600000) / 60000);
  const seconds = Math.floor((duration % 60000) / 1000);

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, "0")}:${seconds
      .toString()
      .padStart(2, "0")}`;
  }
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
};

const getMediaTypeIcon = (type: string) => {
  switch (type.toLowerCase()) {
    case "track":
      return <FaMusic className="text-blue-500" />;
    case "movie":
      return <FaFilm className="text-amber-500 dark:text-amber-300" />;
    case "episode":
      return <FaTv className="text-green-600 dark:text-green-400" />;
    default:
      return <FaPlay className="text-gray-500" />;
  }
};

const getDeviceIcon = (platform: string) => {
  switch (platform.toLowerCase()) {
    case "windows":
    case "macos":
    case "linux":
      return <FaDesktop className="text-gray-600 dark:text-gray-400" />;
    case "ios":
    case "android":
      return <FaMobile className="text-gray-600 dark:text-gray-400" />;
    case "tvos":
    case "roku":
    case "androidtv":
      return <FaTv className="text-gray-600 dark:text-gray-400" />;
    default:
      return <FaTablet className="text-gray-600 dark:text-gray-400" />;
  }
};

const getProgressPercentage = (
  viewOffset: number,
  duration: number
): number => {
  return Math.round((viewOffset / duration) * 100);
};

const formatBitrate = (bitrate: number): string => {
  if (bitrate > 1000) {
    return `${(bitrate / 1000).toFixed(1)} Mbps`;
  }
  return `${bitrate} Kbps`;
};

export const PlexStats: React.FC<PlexStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const [currentOffsets, setCurrentOffsets] = useState<{
    [key: string]: number;
  }>({});
  const service = services.find((s) => s.instanceId === instanceId);
  const isLoading = service?.status === "loading";
  const sessions = useMemo(
    () => service?.stats?.plex?.sessions || [],
    [service?.stats?.plex?.sessions]
  );

  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentOffsets((prev) => {
        const newOffsets = { ...prev };
        sessions.forEach((session: PlexSession) => {
          const sessionKey = `${session.User?.title}-${session.title}`;
          // Only update if playing (not paused)
          if (session.Player?.state === "playing") {
            newOffsets[sessionKey] =
              (prev[sessionKey] || session.viewOffset) + 1000; // Add 1 second (1000ms)
          } else {
            newOffsets[sessionKey] = prev[sessionKey] || session.viewOffset;
          }
        });
        return newOffsets;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [sessions]); // sessions is now properly declared and included in deps

  if (isLoading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="flex items-center space-x-3 bg-gray-50 dark:bg-gray-700/50 p-3 rounded-lg animate-pulse"
          >
            <div className="min-w-0 flex-1">
              <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4 mb-2" />
              <div className="flex space-x-2">
                <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-20" />
                <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-24" />
              </div>
            </div>
            <div className="flex-shrink-0">
              <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-16" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (!service) {
    return null;
  }

  const activeStreams = service.details?.plex?.activeStreams || 0;
  const transcodingCount = service.details?.plex?.transcoding || 0;

  const message = service.health?.message
    ? service.message
      ? `${service.message}\n${service.health.message}`
      : service.health.message
    : service.message;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <PlexMessage status={service.status} message={message} />

      {/* Active Streams Summary */}
      {activeStreams > 0 && (
        <div>
          <div className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
            Active Streams
          </div>

          <div className="rounded-lg bg-white dark:bg-gray-800">
            {/* Summary Stats */}
            <div className="grid grid-cols-2 gap-4 p-4 border-b border-gray-200 dark:border-gray-700">
              <div className="flex flex-col">
                <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                  Total Streams
                </span>
                <span className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                  {activeStreams}
                </span>
              </div>
              <div className="flex flex-col">
                <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                  Transcoding
                </span>
                <span className="text-2xl font-bold text-amber-600 dark:text-amber-400">
                  {transcodingCount}
                </span>
              </div>
            </div>

            {/* Active Sessions */}
            <div>
              {sessions.map((session: PlexSession, index: number) => (
                <div
                  key={index}
                  className="p-4 border-b border-gray-200 dark:border-gray-700 last:border-b-0"
                >
                  {/* Media Title and Type */}
                  <div className="flex items-center space-x-3 mb-3">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center space-x-2">
                        {getMediaTypeIcon(session.type)}
                        <span className="text-sm font-sm text-gray-900 dark:text-gray-100 overflow-hidden">
                          {session.type.toLowerCase() === "movie"
                            ? session.grandparentTitle
                              ? `${session.grandparentTitle} - ${session.title}`
                              : session.title
                            : session.grandparentTitle
                            ? `${session.grandparentTitle} - ${session.title}`
                            : session.title}
                        </span>
                      </div>
                      {session.parentTitle && (
                        <div className="text-xs text-gray-500 dark:text-gray-400">
                          {session.parentTitle}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Progress Bar */}
                  {session.duration && session.viewOffset && (
                    <div className="mb-3">
                      <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1">
                        <div
                          className="bg-blue-500 h-1 rounded-full transition-all duration-300"
                          style={{
                            width: `${getProgressPercentage(
                              currentOffsets[
                                `${session.User?.title}-${session.title}`
                              ] || session.viewOffset,
                              session.duration
                            )}%`,
                          }}
                        />
                      </div>
                      <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400 mt-1">
                        <span>
                          {formatDuration(
                            currentOffsets[
                              `${session.User?.title}-${session.title}`
                            ] || session.viewOffset
                          )}
                        </span>
                        <span>{formatDuration(session.duration)}</span>
                      </div>
                    </div>
                  )}

                  {/* User and Player Info */}
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div className="space-y-2">
                      {session.User && (
                        <div className="flex items-center space-x-2 text-gray-600 dark:text-gray-400">
                          <FaUser className="flex-shrink-0" />
                          <span
                            className="cursor-pointer	"
                            title={session.Player?.address || ""}
                          >
                            {session.User.title}
                          </span>
                        </div>
                      )}
                      {session.Player && (
                        <div className="flex items-center space-x-2 text-gray-600 dark:text-gray-400">
                          {getDeviceIcon(session.Player.platform)}
                          <span className="truncate">
                            {session.Player.product}
                          </span>
                        </div>
                      )}
                    </div>
                    <div className="space-y-2">
                      {session.Media && session.Media[0] && (
                        <>
                          <div className="text-gray-600 dark:text-gray-400">
                            Bitrate: {formatBitrate(session.Media[0].bitrate)}
                          </div>
                          <div className="text-gray-600 dark:text-gray-400">
                            Audio: {session.Media[0].audioCodec.toUpperCase()}{" "}
                            {session.Media[0].audioChannels}ch
                          </div>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
