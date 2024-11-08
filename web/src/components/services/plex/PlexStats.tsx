/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { PlexSession } from "../../../types/service";
import { PlexMessage } from "./PlexMessage";

interface PlexStatsProps {
  instanceId: string;
}

export const PlexStats: React.FC<PlexStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const isLoading = service?.status === "loading";

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

  const sessions = service.stats?.plex?.sessions || [];
  const activeStreams = service.details?.plex?.activeStreams || 0;
  const transcodingCount = service.details?.plex?.transcoding || 0;

  // Combine service message with health message if available
  const message = service.health?.message
    ? service.message
      ? `${service.message}\n${service.health.message}`
      : service.health.message
    : service.message;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <PlexMessage status={service.status} message={message} />

      {/* Active Streams Summary - Always show the header */}
      <div>
        <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
          Active Streams ({activeStreams}):
        </div>

        {/* Only show the details box when there are active streams */}
        {activeStreams > 0 && (
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
            <div className="grid grid-cols-2 gap-4 mb-4">
              <div className="flex flex-col">
                <span className="font-medium text-gray-600 dark:text-gray-300">
                  Total Streams
                </span>
                <span className="text-sm">{activeStreams}</span>
              </div>
              <div className="flex flex-col">
                <span className="font-medium text-gray-600 dark:text-gray-300">
                  Transcoding
                </span>
                <span className="text-sm">{transcodingCount}</span>
              </div>
            </div>

            {/* Active Sessions */}
            {sessions.length > 0 ? (
              <div className="space-y-4">
                {sessions.map((session: PlexSession, index: number) => (
                  <div
                    key={index}
                    className="border-t border-gray-200 dark:border-gray-700 pt-4 first:border-t-0 first:pt-0"
                  >
                    {/* Title Information */}
                    <div className="mb-2">
                      <div className="font-medium text-gray-600 dark:text-gray-300">
                        {session.grandparentTitle
                          ? `${session.grandparentTitle} - ${session.title}`
                          : session.title}
                      </div>
                      <div className="text-xs opacity-75">
                        Type: {session.type}
                      </div>
                    </div>

                    {/* User Information */}
                    {session.User && (
                      <div className="mb-2">
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            User:{" "}
                          </span>
                          {session.User.title}
                        </div>
                      </div>
                    )}

                    {/* Player Information */}
                    {session.Player && (
                      <div className="mb-2">
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            Device:{" "}
                          </span>
                          {session.Player.device} ({session.Player.product})
                        </div>
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            IP:{" "}
                          </span>
                          {session.Player.remotePublicAddress}
                        </div>
                      </div>
                    )}

                    {/* Transcode Information */}
                    {session.TranscodeSession && (
                      <div>
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            Video:{" "}
                          </span>
                          {session.TranscodeSession.videoDecision}
                        </div>
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            Audio:{" "}
                          </span>
                          {session.TranscodeSession.audioDecision}
                        </div>
                        <div className="text-xs">
                          <span className="font-medium text-gray-600 dark:text-gray-300">
                            Progress:{" "}
                          </span>
                          {session.TranscodeSession.progress}%
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center text-gray-500 dark:text-gray-400">
                No active streams
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
