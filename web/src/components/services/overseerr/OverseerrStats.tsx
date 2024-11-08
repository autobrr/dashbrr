/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { OverseerrMessage } from "./OverseerrMessage";
import { OverseerrMediaRequest } from "../../../types/service";

interface OverseerrStatsProps {
  instanceId: string;
}

export const OverseerrStats: React.FC<OverseerrStatsProps> = ({
  instanceId,
}) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const requests = service?.stats?.overseerr?.requests || [];
  const pendingCount = service?.stats?.overseerr?.pendingCount || 0;
  const isLoading = !service || service.status === "loading";
  const error = service?.status === "error" ? service.message : null;

  if (isLoading) {
    return <p className="text-xs text-gray-500">Loading requests...</p>;
  }

  if (error) {
    return <p className="text-xs text-gray-500">Error: {error}</p>;
  }

  // Combine service message with health message if available
  const message = service.health?.message
    ? service.message
      ? `${service.message}\n${service.health.message}`
      : service.health.message
    : service.message;

  const getStatusLabel = (status: number) => {
    switch (status) {
      case 1:
        return "Pending";
      case 2:
        return "Approved";
      case 3:
        return "Declined";
      default:
        return "Unknown";
    }
  };

  const getStatusColor = (status: number) => {
    switch (status) {
      case 1:
        return "text-yellow-500";
      case 2:
        return "text-green-500";
      case 3:
        return "text-red-500";
      default:
        return "text-gray-500";
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getUserDisplayName = (
    requestedBy: OverseerrMediaRequest["requestedBy"]
  ) => {
    if (!requestedBy) return "Unknown User";
    return (
      requestedBy.username ||
      requestedBy.plexUsername ||
      requestedBy.email ||
      "Unknown User"
    );
  };

  const getMediaType = (request: OverseerrMediaRequest) => {
    return request.media.tvdbId ? "TV" : "Movie";
  };

  const getMediaTitle = (request: OverseerrMediaRequest) => {
    if (request.media.title) {
      return request.media.title;
    }
    return request.media.tvdbId
      ? `TV Show (TVDB: ${request.media.tvdbId})`
      : `Movie (TMDB: ${request.media.tmdbId})`;
  };

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <OverseerrMessage status={service.status} message={message} />

      {/* Pending Requests Count */}
      <div>
        <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
          Pending Requests:
        </div>
        <div className="text-xs rounded-md text-gray-700 dark:text-gray-400 bg-gray-850/95 p-4">
          {pendingCount} {pendingCount === 1 ? "request" : "requests"} awaiting
          approval
        </div>
      </div>

      {/* Recent Requests List */}
      {requests.length > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            Recent Requests:
          </div>
          <div className="text-xs rounded-md text-gray-700 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
            {requests.slice(0, 5).map((request: OverseerrMediaRequest) => (
              <div
                key={request.id}
                className="flex justify-between items-start border-b border-gray-800 last:border-0 pb-2 last:pb-0"
              >
                <div className="flex-1">
                  <div className="font-medium flex items-center gap-2">
                    {getMediaTitle(request)}
                    <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-gray-700/70 text-gray-300">
                      {getMediaType(request)}
                    </span>
                  </div>
                  <div className="text-gray-500 flex items-center gap-2">
                    <span>{getUserDisplayName(request.requestedBy)}</span>
                    <span>•</span>
                    <span>{formatDate(request.createdAt)}</span>
                  </div>
                </div>
                <div
                  className={`${getStatusColor(
                    request.status
                  )} text-xs font-medium ml-2`}
                >
                  {getStatusLabel(request.status)}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
