/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState, useEffect, useMemo } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { PlexSession } from "../../../types/service";
import { ArrMessage } from "../common/ArrMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import {
  FaUser,
  FaPlay,
  FaPause,
  FaMusic,
  FaFilm,
  FaTv,
  FaDesktop,
  FaMobile,
  FaTablet,
  FaExchangeAlt,
  FaPlayCircle,
} from "react-icons/fa";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import { CollapsibleSection } from "../../ui/CollapsibleSection";

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

const getMediaTypeIcon = (
  type: string | undefined,
  playerState: string | undefined
) => {

  if (playerState?.toLowerCase() === "paused") {
    return <FaPause className="text-gray-500 dark:text-gray-400 h-4 w-4" />;
  }

  switch (type?.toLowerCase()) {
    case "track":
      return <FaMusic className="text-blue-600 dark:text-blue-400 h-4 w-4" />;
    case "movie":
      return <FaFilm className="text-amber-500 dark:text-amber-300 h-4 w-4" />;
    case "episode":
      return <FaTv className="text-green-600 dark:text-green-400 h-4 w-4" />;
    case "clip":
      return <FaPlayCircle className="text-purple-500 dark:text-purple-400 h-4 w-4" />;
    default:
      return <FaPlay className="text-gray-500 h-4 w-4" />;
  }
};

const getDeviceIcon = (platform: string) => {
  switch (platform.toLowerCase()) {
    case "windows":
    case "macos":
    case "linux":
      return <FaDesktop className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    case "ios":
    case "android":
      return <FaMobile className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    case "tvos":
    case "roku":
    case "androidtv":
      return <FaTv className="text-gray-600 dark:text-gray-400 w-4 h-4" />;
    default:
      return <FaTablet className="text-gray-600 dark:text-gray-400 w-4" />;
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

const isTranscoding = (session: PlexSession): boolean => {
  return (
    session.TranscodeSession?.videoDecision === "transcode" ||
    session.TranscodeSession?.audioDecision === "transcode" ||
    session.TranscodeSession?.audioCodec?.toLowerCase() === "opus"
  );
};

interface TimerState {
  offset: number;
  lastUpdated: number;
  state: string;
}

const getPlaybackKey = (session: PlexSession): string => {
  return (
    session.sessionKey ||
    session.Session?.id ||
    session.key ||
    `${session.User?.title || "unknown"}:${session.guid}`
  );
};

export const PlexStats: React.FC<PlexStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const [playbackStates, setPlaybackStates] = useState<{
    [key: string]: TimerState;
  }>({});
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "plex:active_streams"),
    true
  );
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";
  const sessions = useMemo(
    () => service?.stats?.plex?.sessions || [],
    [service?.stats?.plex?.sessions]
  );

  const transcodingCount = useMemo(
    () => sessions.filter((session) => isTranscoding(session)).length,
    [sessions]
  );

  useEffect(() => {
    // Derive next timer state from previous state + latest sessions without
    // re-subscribing the effect on every tick/state update.
    setPlaybackStates((prev) => {
      const currentTime = Date.now();
      const next: { [key: string]: TimerState } = {};

      for (const session of sessions) {
        const sessionKey = getPlaybackKey(session);
        const existing = prev[sessionKey];
        const state = session.Player?.state || "stopped";

        if (existing) {
          const offset =
            existing.state === "playing"
              ? existing.offset + (currentTime - existing.lastUpdated)
              : existing.offset;

          next[sessionKey] = {
            offset,
            lastUpdated: currentTime,
            state,
          };
        } else {
          next[sessionKey] = {
            offset: session.viewOffset || 0,
            lastUpdated: currentTime,
            state,
          };
        }
      }

      return next;
    });

    const timer = setInterval(() => {
      setPlaybackStates((prev) => {
        const currentTime = Date.now();
        const updated = { ...prev };

        Object.keys(updated).forEach((key) => {
          const state = updated[key];
          if (state.state === "playing") {
            const timeDiff = currentTime - state.lastUpdated;
            updated[key] = {
              ...state,
              offset: state.offset + timeDiff,
              lastUpdated: currentTime,
            };
          }
        });

        return updated;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [sessions]);

  const getCurrentOffset = (session: PlexSession): number => {
    const sessionKey = getPlaybackKey(session);
    const state = playbackStates[sessionKey];

    if (!state) return session.viewOffset || 0;

    if (state.state === "playing") {
      const timeDiff = Date.now() - state.lastUpdated;
      return state.offset + timeDiff;
    }

    return state.offset;
  };

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const activeStreams = service.details?.plex?.activeStreams || 0;

  const message = combineServiceMessage(service);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      {activeStreams > 0 && (
        <div>
          <CollapsibleSection
            title={
              <div className="flex w-full items-center justify-between pr-6">
                <div className="text-xs font-medium text-gray-700 dark:text-gray-300">
                  Active Streams:
                </div>
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      Total:
                    </span>
                    <span className="text-xs font-medium text-blue-500 dark:text-blue-400">
                      {activeStreams}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      Transcoding:
                    </span>
                    <span className="text-xs font-medium text-amber-500 dark:text-amber-400">
                      {transcodingCount}
                    </span>
                  </div>
                </div>
              </div>
            }
            isExpanded={isExpanded}
            onToggle={toggle}
          >
            <div className="space-y-2">
              {sessions.map((session: PlexSession) => (
                <div
                  key={getPlaybackKey(session)}
                  className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5 hover:bg-gray-850/80 transition-colors"
                >
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center justify-center gap-2">
                      <span className={session.Player?.state?.toLowerCase() === "paused" ? "text-yellow-500" : "text-blue-500"}>
                        {getMediaTypeIcon(session.type || "", session.Player?.state)}
                      </span>
                      <div className="flex items-center justify-between flex-1">
                        <span className="text-xs font-medium text-gray-200 truncate" title={session.title}>
                          {session.type?.toLowerCase() === "movie"
                            ? session.grandparentTitle
                              ? `${session.grandparentTitle} - ${session.title}`
                              : session.title
                            : session.grandparentTitle
                            ? `${session.grandparentTitle} - ${session.title}`
                            : session.title ?? ""} 
                          {session.type?.toLowerCase() === "clip" && (
                            <span className="text-purple-400 ml-1">(Trailer)</span>
                          )}
                        </span>
                        {isTranscoding(session) && (
                          <div className="flex items-center gap-1 ml-4 text-amber-500 dark:text-amber-400">
                            <FaExchangeAlt className="h-3 w-3" />
                            <span className="text-[10px]">Transcoding</span>
                          </div>
                        )}
                      </div>
                    </div>

                    {session.duration > 0 && (
                      <div className="px-6">
                        <div className="w-full bg-gray-700/50 rounded-full h-1">
                          <div
                            className="bg-blue-500 h-1 rounded-full transition-all duration-300"
                            style={{
                              width: `${getProgressPercentage(
                                getCurrentOffset(session),
                                session.duration
                              )}%`,
                            }}
                          />
                        </div>
                        <div className="flex justify-between text-[10px] text-gray-400 mt-1">
                          <span>{formatDuration(getCurrentOffset(session))}</span>
                          <span>{formatDuration(session.duration)}</span>
                        </div>
                      </div>
                    )}

                    <div className="flex items-center justify-between -ml-2">
                      <div className="flex flex-wrap items-center gap-y-1.5 text-xs text-gray-400">
                        {session.User && (
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            <FaUser className="h-4 w-4 text-gray-400" />
                            <span title={session.Player?.address || ""}>{session.User.title}</span>
                          </span>
                        )}
                        {session.Player && (
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            {getDeviceIcon(session.Player.platform)}
                            <span className="truncate">{session.Player.product}</span>
                          </span>
                        )}
                      </div>
                      {session.Media && session.Media[0] && (
                        <div className="flex items-center gap-2 text-[10px]">
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            {formatBitrate(session.Media[0].bitrate)}
                          </span>
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            {session.Media[0].audioCodec.toUpperCase()} {session.Media[0].audioChannels}ch
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CollapsibleSection>
        </div>
      )}
    </div>
  );
};
