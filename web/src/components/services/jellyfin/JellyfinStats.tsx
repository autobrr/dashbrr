/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useState } from "react";
import { FaExchangeAlt, FaPause, FaPlay, FaUser } from "react-icons/fa";

import { useServiceData } from "../../../hooks/useServiceData";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { ArrMessage } from "../common/ArrMessage";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import type {
  JellyfinMediaStream,
  JellyfinSession,
  JellyfinSummary,
} from "../../../types/service";
import {
  formatBitrateBps,
  formatDurationTicks,
  getDeviceIcon,
  getMediaTypeIcon,
  getProgressPercentage,
} from "../common/playbackUi";

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
const EMPTY_SESSIONS: JellyfinSession[] = [];

const getSessionTitle = (name?: string, seriesName?: string): string => {
  if (seriesName && name) {
    return `${seriesName} - ${name}`;
  }
  return name || seriesName || "Unknown playback";
};

const getPlayStateText = (isPaused?: boolean): string => (isPaused ? "Paused" : "Playing");

const isPlayingSession = (session: JellyfinSession): boolean =>
  !session.PlayState?.IsPaused &&
  Boolean(session.NowPlayingItem) &&
  (session.IsActive || (session.PlayState?.PositionTicks ?? 0) > 0);

const isTranscodingSession = (session: JellyfinSession): boolean =>
  Boolean(session.TranscodingInfo) ||
  (session.PlayState?.PlayMethod || "").toLowerCase() === "transcode";

const isLikelyVideoStream = (stream: JellyfinMediaStream): boolean =>
  (stream.Width || 0) > 0 || (stream.Height || 0) > 0;

const getAudioStream = (session: JellyfinSession): JellyfinMediaStream | undefined => {
  const streams = session.NowPlayingItem?.MediaStreams ?? [];
  const streamIndex = session.PlayState?.AudioStreamIndex;

  if (typeof streamIndex === "number") {
    const selected = streams.find((stream) => stream.Index === streamIndex);
    if (selected) {
      return selected;
    }
  }

  const byChannels = streams.find((stream) => (stream.Channels || 0) > 0);
  if (byChannels) {
    return byChannels;
  }

  return streams.find((stream) => !isLikelyVideoStream(stream) && Boolean(stream.Codec));
};

const getVideoStream = (session: JellyfinSession): JellyfinMediaStream | undefined => {
  const streams = session.NowPlayingItem?.MediaStreams ?? [];
  return streams.find((stream) => isLikelyVideoStream(stream));
};

const formatCodecWithChannels = (codec?: string, channels?: number): string => {
  const normalizedCodec = (codec || "").trim().toUpperCase();
  if (!normalizedCodec) {
    return "";
  }
  if (channels && channels > 0) {
    return `${normalizedCodec} ${channels}ch`;
  }
  return normalizedCodec;
};

interface TimerState {
  offsetTicks: number;
  lastUpdated: number;
  isPaused: boolean;
}

const getPlaybackKey = (session: JellyfinSession): string =>
  session.Id ||
  `${session.UserName || "unknown"}:${session.NowPlayingItem?.SeriesName || ""}:${session.NowPlayingItem?.Name || ""}`;

export const JellyfinStats: React.FC<JellyfinStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.jellyfin?.summary ?? EMPTY_SUMMARY;
  const sessions = summary.sessions ?? EMPTY_SESSIONS;
  const [playbackStates, setPlaybackStates] = useState<Record<string, TimerState>>({});

  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "jellyfin:active_streams"),
    true
  );

  useEffect(() => {
    setPlaybackStates((prev) => {
      const now = Date.now();
      const next: Record<string, TimerState> = {};

      for (const session of sessions) {
        const key = getPlaybackKey(session);
        const existing = prev[key];
        const currentOffset = session.PlayState?.PositionTicks ?? 0;
        const isPaused = Boolean(session.PlayState?.IsPaused);

        if (existing) {
          const offsetTicks =
            !existing.isPaused && isPlayingSession(session)
              ? existing.offsetTicks + Math.floor((now - existing.lastUpdated) * 10000)
              : currentOffset;
          next[key] = { offsetTicks, lastUpdated: now, isPaused };
          continue;
        }

        next[key] = {
          offsetTicks: currentOffset,
          lastUpdated: now,
          isPaused,
        };
      }

      return next;
    });

    const timer = window.setInterval(() => {
      setPlaybackStates((prev) => {
        const now = Date.now();
        const next = { ...prev };

        for (const session of sessions) {
          const key = getPlaybackKey(session);
          const current = next[key];
          if (!current || current.isPaused || !isPlayingSession(session)) {
            continue;
          }

          const elapsedMs = now - current.lastUpdated;
          next[key] = {
            ...current,
            offsetTicks: current.offsetTicks + Math.floor(elapsedMs * 10000),
            lastUpdated: now,
          };
        }

        return next;
      });
    }, 1000);

    return () => {
      window.clearInterval(timer);
    };
  }, [sessions]);

  const getCurrentOffsetTicks = (session: JellyfinSession): number => {
    const key = getPlaybackKey(session);
    const state = playbackStates[key];
    return state ? state.offsetTicks : session.PlayState?.PositionTicks ?? 0;
  };

  const isInitialLoading =
    service?.status === "loading" && !service?.stats?.jellyfin?.summary;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);
  const activeStreams = service.details?.jellyfin?.activeStreams ?? sessions.length;
  const transcoding =
    service.details?.jellyfin?.transcoding ??
    sessions.filter((session) => isTranscodingSession(session)).length;
  const paused =
    service.details?.jellyfin?.paused ??
    sessions.filter((session) => Boolean(session.PlayState?.IsPaused)).length;

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
                      {transcoding}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      Paused:
                    </span>
                    <span className="text-xs font-medium text-zinc-300">{paused}</span>
                  </div>
                </div>
              </div>
            }
            isExpanded={isExpanded}
            onToggle={toggle}
          >
            <div className="space-y-2">
              {sessions.map((session) => {
                const runtimeTicks = session.NowPlayingItem?.RunTimeTicks ?? 0;
                const currentOffsetTicks = getCurrentOffsetTicks(session);
                const progress = getProgressPercentage(currentOffsetTicks, runtimeTicks);
                const transcodingInfo = session.TranscodingInfo;
                const isPaused = Boolean(session.PlayState?.IsPaused);
                const audioStream = getAudioStream(session);
                const videoStream = getVideoStream(session);
                const transcodeAudioLabel = formatCodecWithChannels(
                  transcodingInfo?.AudioCodec,
                  transcodingInfo?.AudioChannels
                );
                const transcodeVideoCodec = (transcodingInfo?.VideoCodec || "")
                  .trim()
                  .toUpperCase();
                const directAudioLabel = formatCodecWithChannels(
                  audioStream?.Codec,
                  audioStream?.Channels
                );
                const metadataLabel =
                  transcodeAudioLabel || transcodeVideoCodec
                    ? `${transcodeAudioLabel || "AUDIO"}${transcodeVideoCodec ? ` · ${transcodeVideoCodec}` : ""}`
                    : directAudioLabel;
                const displayBitrate =
                  transcodingInfo?.Bitrate ||
                  videoStream?.BitRate ||
                  audioStream?.BitRate ||
                  0;

                return (
                  <div
                    key={getPlaybackKey(session)}
                    className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5 hover:bg-gray-850/80 transition-colors"
                  >
                    <div className="flex flex-col gap-2">
                      <div className="flex items-center justify-center gap-2">
                        <span className={isPaused ? "text-yellow-500" : "text-blue-500"}>
                          {getMediaTypeIcon(session.NowPlayingItem?.Type, isPaused)}
                        </span>
                        <div className="flex items-center justify-between flex-1 gap-2">
                          <span
                            className="text-xs font-medium text-gray-200 truncate"
                            title={getSessionTitle(
                              session.NowPlayingItem?.Name,
                              session.NowPlayingItem?.SeriesName
                            )}
                          >
                            {getSessionTitle(
                              session.NowPlayingItem?.Name,
                              session.NowPlayingItem?.SeriesName
                            )}
                          </span>
                          {isTranscodingSession(session) && (
                            <div className="flex items-center gap-1 ml-2 text-amber-500 dark:text-amber-400">
                              <FaExchangeAlt className="h-3 w-3" />
                              <span className="text-[10px]">Transcoding</span>
                            </div>
                          )}
                          <span className="text-[10px] text-zinc-400 ml-1">
                            {isPaused ? (
                              <span className="inline-flex items-center gap-1">
                                <FaPause className="h-2.5 w-2.5" /> Paused
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1">
                                <FaPlay className="h-2.5 w-2.5" /> Playing
                              </span>
                            )}
                          </span>
                        </div>
                      </div>

                      {runtimeTicks > 0 && (
                        <div className="px-6">
                          <div className="w-full bg-gray-700/50 rounded-full h-1">
                            <div
                              className="bg-blue-500 h-1 rounded-full transition-all duration-300"
                              style={{ width: `${progress}%` }}
                            />
                          </div>
                          <div className="flex justify-between text-[10px] text-gray-400 mt-1">
                            <span>{formatDurationTicks(currentOffsetTicks)}</span>
                            <span>{formatDurationTicks(runtimeTicks)}</span>
                          </div>
                        </div>
                      )}

                      <div className="flex items-center justify-between -ml-2">
                        <div className="flex flex-wrap items-center gap-y-1.5 text-xs text-gray-400">
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            <FaUser className="h-4 w-4 text-gray-400" />
                            <span>{session.UserName || "Unknown user"}</span>
                          </span>
                          <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                            {getDeviceIcon(session.DeviceType || session.Client)}
                            <span className="truncate">
                              {session.Client || session.DeviceName || "Unknown client"}
                            </span>
                          </span>
                        </div>

                        <div className="flex items-center gap-2 text-[10px]">
                          {displayBitrate > 0 ? (
                            <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                              {formatBitrateBps(displayBitrate)}
                            </span>
                          ) : null}
                          {metadataLabel ? (
                            <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                              {metadataLabel}
                            </span>
                          ) : (
                            <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                              {session.PlayState?.PlayMethod || getPlayStateText(isPaused)}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </CollapsibleSection>
        </div>
      )}
    </div>
  );
};
