/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState } from "react";
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
  ChevronUpIcon,
} from "@heroicons/react/24/outline";
import { AutobrrRelease } from "../../../types/service";
import { getMediaType, getMediaTypeIcon } from "../../../utils/mediaTypes";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface AutobrrStatsProps {
  instanceId: string;
}

export const AutobrrStats: React.FC<AutobrrStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const isLoading = service?.status === "loading";
  const [isExpanded, setIsExpanded] = useState(true);

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
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
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
            Stats:
          </div>
          <div className="grid grid-cols-2 gap-1.5">
            <a
              href={getReleasesUrl()}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2 hover:bg-gray-850/70 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-xs font-normal text-gray-200">Total</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold text-gray-200">{stats.total_count || 0}</span>
                <LinkIcon className="h-3 w-3 text-gray-400" />
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_APPROVED")}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2 hover:bg-gray-850/70 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-xs font-normal text-gray-200">Approved</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold text-green-500">{stats.push_approved_count || 0}</span>
                <LinkIcon className="h-3 w-3 text-gray-400" />
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_REJECTED")}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2 hover:bg-gray-850/70 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-xs font-normal text-gray-200">Rejected</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold text-blue-400">{stats.push_rejected_count || 0}</span>
                <LinkIcon className="h-3 w-3 text-gray-400" />
              </div>
            </a>

            <a
              href={getReleasesUrl("PUSH_ERROR")}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2 hover:bg-gray-850/70 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-xs font-normal text-gray-200">Errors</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold text-red-500">{stats.push_error_count || 0}</span>
                <LinkIcon className="h-3 w-3 text-gray-400" />
              </div>
            </a>
          </div>
        </div>
      )}

      {/* Recent Releases */}
      {(service.status === "online" || service.status === "warning") && (
        <div>
          <div
            onClick={() => setIsExpanded(!isExpanded)}
            className="relative cursor-pointer select-none w-full flex items-center justify-between text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 group"
          >
            <span>Recent Releases:</span>
            <div className="absolute pr-0.5 right-0 top-1/2 -translate-y-1/2 transition-transform duration-200 text-gray-500">
              <ChevronUpIcon
                className={`h-3.5 w-3.5 transform transition-transform duration-200 ${
                  isExpanded ? "rotate-180" : ""
                } group-hover:text-gray-400`}
              />
            </div>
          </div>
          <div
            className={`overflow-hidden transition-[max-height,opacity] duration-200 ease-in-out ${
              isExpanded ? "max-h-[1000px] opacity-100" : "max-h-0 opacity-0"
            }`}
          >
            {releases.length > 0 ? (
              <div className="space-y-2">
                {releases.slice(0, 5).map((release: AutobrrRelease) => (
                  <div
                    key={release.id}
                    className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5 hover:bg-gray-850/70 transition-colors"
                  >
                    <div className="flex flex-col gap-2">
                      <div className="flex items-center gap-2">
                        {release.filter_status === "FILTER_REJECTED" ? (
                          <span className="text-red-500">
                            <NoSymbolIcon className="h-4 w-4" />
                          </span>
                        ) : release.action_status?.[0]?.status === "PUSH_APPROVED" ? (
                          <span className="text-green-500">
                            <CheckCircleIcon className="h-4 w-4" />
                          </span>
                        ) : release.action_status?.[0]?.status === "PUSH_REJECTED" ? (
                          <span className="text-blue-400">
                            <XCircleIcon className="h-4 w-4" />
                          </span>
                        ) : release.action_status?.[0]?.status === "PUSH_ERROR" ? (
                          <span className="text-red-500">
                            <ExclamationCircleIcon className="h-4 w-4" />
                          </span>
                        ) : (
                          <span className="text-yellow-500">
                            <ClockIcon className="h-4 w-4" />
                          </span>
                        )}
                        <span className="text-xs font-medium text-gray-200 truncate flex-1" title={release.name}>
                          {release.name}
                        </span>
                        <div className="flex items-center gap-1 flex-shrink-0">
                          {release.download_url && (
                            <a
                              href={release.download_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-blue-400 hover:text-blue-300 p-1 hover:bg-gray-700/50 rounded transition-colors"
                              title={`Download torrentfile`}
                            >
                              <ArrowDownTrayIcon className="h-3.5 w-3.5" />
                            </a>
                          )}
                          {release.info_url && (
                            <a
                              href={release.info_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-blue-400 hover:text-blue-300 p-1 hover:bg-gray-700/50 rounded transition-colors"
                              title={`View this release on ${release.indexer.name}`}
                            >
                              <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5" />
                            </a>
                          )}
                        </div>
                      </div>
                      <div className="flex flex-wrap items-center justify-between gap-y-1 text-xs text-gray-400">
                        <div className="flex flex-wrap items-center gap-x-2">
                          {release.source && (
                            <span className="flex items-center gap-1 bg-gray-800/50 py-0.5 rounded">
                              {(() => {
                                const mediaType = getMediaType(release.category);
                                const IconComponent = getMediaTypeIcon(mediaType);
                                return (
                                  <>
                                    <IconComponent className="h-4 w-4 text-gray-400" />
                                    {mediaType}
                                  </>
                                );
                              })()}
                            </span>
                          )}
                          <span className="flex items-center gap-1 bg-gray-800/50 px-1.5 py-0.5 rounded">
                            {release.indexer.name}
                          </span>
                          {release.filter && (
                            <span className="bg-gray-800/50 px-1.5 py-0.5 rounded">
                              {release.filter}
                            </span>
                          )}
                        </div>
                        {release.action_status?.[0]?.status && (
                          <span className={`bg-gray-800/50 px-1.5 py-0.5 rounded ${
                            release.action_status[0].status === "PUSH_APPROVED"
                              ? "text-green-500"
                              : release.action_status[0].status === "PUSH_REJECTED"
                              ? "text-blue-400"
                              : release.action_status[0].status === "PUSH_ERROR"
                              ? "text-red-500"
                              : "text-yellow-500"
                          }`}>
                            {release.action_status[0].status.replace("PUSH_", "")
                              .toLowerCase()
                              .replace(/^\w/, (c) => c.toUpperCase())}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5">
                No recent releases
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
