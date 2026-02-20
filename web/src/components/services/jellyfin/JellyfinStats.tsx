/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { FaExchangeAlt, FaPause, FaPlay, FaTv, FaUser } from "react-icons/fa";

import { useServiceData } from "../../../hooks/useServiceData";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { ArrMessage } from "../common/ArrMessage";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import type { JellyfinSummary } from "../../../types/service";

interface JellyfinStatsProps {
  instanceId: string;
}

const EMPTY_SUMMARY: JellyfinSummary = {
  system: {
    ServerName: "",
    Version: "",
    ProductName: "",
    Id: "",
  },
  sessions: [],
};

const TICKS_PER_SECOND = 10_000_000;

const formatTicks = (ticks?: number): string => {
  if (!ticks || ticks <= 0) return "0:00";
  const totalSeconds = Math.floor(ticks / TICKS_PER_SECOND);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
};

const getSessionTitle = (name?: string, seriesName?: string): string => {
  if (seriesName && name) {
    return `${seriesName} - ${name}`;
  }
  return name || seriesName || "Unknown playback";
};

const getPlayStateText = (isPaused?: boolean): string =>
  isPaused ? "Paused" : "Playing";

export const JellyfinStats: React.FC<JellyfinStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.jellyfin?.summary ?? EMPTY_SUMMARY;

  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "jellyfin:active_streams"),
    true
  );

  const isInitialLoading =
    service?.status === "loading" && !service?.stats?.jellyfin?.summary;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);
  const activeStreams = service.details?.jellyfin?.activeStreams ?? summary.sessions.length;
  const transcoding =
    service.details?.jellyfin?.transcoding ??
    summary.sessions.filter((session) => Boolean(session.TranscodingInfo)).length;
  const paused =
    service.details?.jellyfin?.paused ??
    summary.sessions.filter((session) => Boolean(session.PlayState?.IsPaused)).length;

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      {activeStreams > 0 && (
        <CollapsibleSection
          title={
            <div className="flex w-full items-center justify-between pr-6">
              <div className="text-xs font-medium text-gray-700 dark:text-gray-300">
                Active Streams:
              </div>
              <div className="flex items-center gap-4 text-xs">
                <span className="text-gray-500 dark:text-gray-400">
                  Total: <span className="font-medium text-blue-500 dark:text-blue-400">{activeStreams}</span>
                </span>
                <span className="text-gray-500 dark:text-gray-400">
                  Transcoding: <span className="font-medium text-amber-500 dark:text-amber-400">{transcoding}</span>
                </span>
                <span className="text-gray-500 dark:text-gray-400">
                  Paused: <span className="font-medium text-zinc-300">{paused}</span>
                </span>
              </div>
            </div>
          }
          isExpanded={isExpanded}
          onToggle={toggle}
        >
          <div className="space-y-2">
            {summary.sessions.map((session) => {
              const runtimeTicks = session.NowPlayingItem?.RunTimeTicks ?? 0;
              const positionTicks = session.PlayState?.PositionTicks ?? 0;
              const progress =
                runtimeTicks > 0
                  ? Math.max(0, Math.min(100, Math.round((positionTicks / runtimeTicks) * 100)))
                  : 0;

              return (
                <div
                  key={session.Id}
                  className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5 hover:bg-gray-850/80 transition-colors"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <FaTv className="text-blue-500 h-3.5 w-3.5" />
                      <span
                        className="text-xs font-medium text-gray-200 truncate"
                        title={getSessionTitle(session.NowPlayingItem?.Name, session.NowPlayingItem?.SeriesName)}
                      >
                        {getSessionTitle(session.NowPlayingItem?.Name, session.NowPlayingItem?.SeriesName)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      {session.TranscodingInfo && (
                        <span className="flex items-center gap-1 text-amber-400">
                          <FaExchangeAlt className="h-3 w-3" />
                          Transcoding
                        </span>
                      )}
                      <span className="text-zinc-400">
                        {session.PlayState?.IsPaused ? (
                          <span className="inline-flex items-center gap-1"><FaPause className="h-3 w-3" /> Paused</span>
                        ) : (
                          <span className="inline-flex items-center gap-1"><FaPlay className="h-3 w-3" /> Playing</span>
                        )}
                      </span>
                    </div>
                  </div>

                  {runtimeTicks > 0 && (
                    <div className="mt-2 px-0.5">
                      <div className="w-full bg-gray-700/50 rounded-full h-1">
                        <div
                          className="bg-blue-500 h-1 rounded-full transition-all duration-300"
                          style={{ width: `${progress}%` }}
                        />
                      </div>
                      <div className="flex justify-between text-[10px] text-gray-400 mt-1">
                        <span>{formatTicks(positionTicks)}</span>
                        <span>{formatTicks(runtimeTicks)}</span>
                      </div>
                    </div>
                  )}

                  <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-[11px] text-gray-400">
                    <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                      <FaUser className="h-3 w-3" />
                      {session.UserName || "Unknown user"}
                    </span>
                    <span className="bg-gray-800/50 px-2 py-0.5 rounded truncate max-w-[45%]">
                      {session.Client || session.DeviceName || "Unknown client"}
                    </span>
                    <span className="bg-gray-800/50 px-2 py-0.5 rounded">
                      {session.PlayState?.PlayMethod || getPlayStateText(session.PlayState?.IsPaused)}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
