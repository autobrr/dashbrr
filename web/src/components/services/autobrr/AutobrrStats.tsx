/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { StatusIcon } from "../../ui/StatusIcon";
import { AutobrrMessage } from "./AutobrrMessage";
import {
  ArrowDownTrayIcon,
  ArrowTopRightOnSquareIcon,
  LinkIcon,
  CheckCircleIcon,
  XCircleIcon,
  ExclamationCircleIcon,
  NoSymbolIcon,
  ClockIcon,
} from "@heroicons/react/24/solid";
import { AutobrrRelease } from "../../../types/service";
import { getMediaType, getMediaTypeIcon } from "../../../utils/mediaTypes";

interface AutobrrStatsProps {
  instanceId: string;
}

export const AutobrrStats: React.FC<AutobrrStatsProps> = ({ instanceId }) => {
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

  // Always show stats section if service is online, even if stats are empty
  const showStats = true;
  const stats = service.stats?.autobrr || {
    total_count: 0,
    filtered_count: 0,
    filter_rejected_count: 0,
    push_approved_count: 0,
    push_rejected_count: 0,
    push_error_count: 0,
  };
  const ircStatus = service.details?.autobrr?.irc;
  const releases = service.releases?.data || [];

  console.log("Service releases:", service.instanceId, service.releases?.data);

  // Only show message component if there's a message or status isn't online
  const showMessage = service.message || service.status !== "online";

  const baseUrl = service?.url || "";

  // Function to construct the full URL for releases
  const getReleasesUrl = (actionStatus?: string) => {
    const url = new URL("releases", baseUrl);
    if (actionStatus) {
      url.searchParams.set("action_status", actionStatus);
    }
    return url.toString();
  };

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      {showMessage && (
        <AutobrrMessage status={service.status} message={service.message} />
      )}

      {/* IRC Status */}
      {ircStatus && ircStatus.some((irc) => !irc.healthy) && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            IRC Status:
          </div>
          <div className="text-sm rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-1">
            {ircStatus.map((irc, index) => (
              <div key={index} className="flex justify-between items-center">
                <span className="font-medium text-xs">{irc.name}</span>
                <div className="flex items-center">
                  <StatusIcon status={irc.healthy ? "online" : "error"} />
                  <span className="ml-2 text-xs" style={{ color: "inherit" }}>
                    {irc.healthy ? "Healthy" : "Unhealthy"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Stats */}
      {showStats && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            Stats:
          </div>
          <div className="grid grid-cols-2 gap-2">
            <a
              href={getReleasesUrl()}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:opacity-80 transition-opacity"
            >
              <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">Total Releases</span>
                  <LinkIcon className="h-3 w-3 text-gray-400" />
                </div>
                <div className="mt-2 text-lg font-bold text-white/80">
                  {stats.total_count || 0}
                </div>
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_APPROVED")}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:opacity-80 transition-opacity"
            >
              <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">Approved pushes</span>
                  <LinkIcon className="h-3 w-3 text-gray-400" />
                </div>
                <div className="mt-2 text-lg font-bold text-green-500/80">
                  {stats.push_approved_count || 0}
                </div>
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_REJECTED")}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:opacity-80 transition-opacity"
            >
              <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">Rejected pushes</span>
                  <LinkIcon className="h-3 w-3 text-gray-400" />
                </div>
                <div className="mt-2 text-lg font-bold text-blue-400/80">
                  {stats.push_rejected_count || 0}
                </div>
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_ERROR")}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:opacity-80 transition-opacity"
            >
              <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">Errored pushes</span>
                  <LinkIcon className="h-3 w-3 text-gray-400" />
                </div>
                <div className="mt-2 text-xl font-bold text-red-500/80">
                  {stats.push_error_count || 0}
                </div>
              </div>
            </a>
          </div>
        </div>
      )}

      {/* Recent Releases */}
      {releases.length > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            Recent Releases:
          </div>
          <div className="space-y-1.5">
            {releases.slice(0, 5).map((release: AutobrrRelease) => (
              <div
                key={release.id}
                className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3"
              >
                <div className="flex justify-between items-center gap-3">
                  <div className="flex-1 min-w-0">
                    <div
                      className="text-xs rounded-md text-gray-700 dark:text-gray-400 truncate space-y-2 cursor-help"
                      title={release.name}
                    >
                      {release.name}
                    </div>
                    <div className="mt-1 text-xs flex flex-wrap items-center gap-x-3 gap-y-1">
                      <div className="flex items-center gap-1.5">
                        {release.filter_status === "FILTER_REJECTED" ? (
                          <NoSymbolIcon className="w-4 h-4 text-red-500" />
                        ) : release.action_status?.[0]?.status ===
                          "PUSH_APPROVED" ? (
                          <CheckCircleIcon className="w-4 h-4 text-green-500" />
                        ) : release.action_status?.[0]?.status ===
                          "PUSH_REJECTED" ? (
                          <XCircleIcon className="w-4 h-4 text-blue-400" />
                        ) : release.action_status?.[0]?.status ===
                          "PUSH_ERROR" ? (
                          <ExclamationCircleIcon className="w-4 h-4 text-red-500" />
                        ) : (
                          <ClockIcon className="w-4 h-4 text-yellow-500" />
                        )}
                        <span>{release.indexer.name}</span>
                      </div>
                      <span className="text-gray-500">•</span>
                      <span>
                        {release.filter_status === "FILTER_REJECTED"
                          ? "Rejected"
                          : release.action_status?.[0]?.status ===
                            "PUSH_APPROVED"
                          ? "Approved"
                          : release.action_status?.[0]?.status ===
                            "PUSH_REJECTED"
                          ? "Rejected"
                          : release.action_status?.[0]?.status === "PUSH_ERROR"
                          ? "Error"
                          : "Pending"}
                      </span>
                      {release.filter && (
                        <>
                          <span className="text-gray-500 truncate">•</span>
                          <span>{release.filter}</span>
                        </>
                      )}
                      {release.source && (
                        <>
                          <span className="text-gray-500">•</span>
                          <span className="flex items-center gap-1">
                            {(() => {
                              const mediaType = getMediaType(release.category);
                              const IconComponent = getMediaTypeIcon(mediaType);
                              return (
                                <>
                                  <IconComponent className="w-4 h-4" />
                                  {mediaType}
                                </>
                              );
                            })()}
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                  {release.download_url && (
                    <a
                      href={release.download_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:text-blue-300 flex-shrink-0"
                      title={`Download torrentfile`}
                    >
                      <ArrowDownTrayIcon className="h-3.5 w-3.5" />
                    </a>
                  )}{" "}
                  {release.info_url && (
                    <a
                      href={release.info_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:text-blue-300 flex-shrink-0"
                      title={`View this release on ${release.indexer.name}`}
                    >
                      <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5" />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
