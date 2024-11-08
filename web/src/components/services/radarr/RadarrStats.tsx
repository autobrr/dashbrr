/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { RadarrQueueItem } from "../../../types/service";
import { RadarrMessage } from "./RadarrMessage";

interface RadarrStatsProps {
  instanceId: string;
}

export const RadarrStats: React.FC<RadarrStatsProps> = ({ instanceId }) => {
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

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <RadarrMessage status={service.status} message={service.message} />

      {/* Queue Display */}
      {service.stats?.radarr?.queue &&
        service.stats.radarr.queue.totalRecords > 0 && (
          <div>
            <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
              Queue ({service.stats.radarr.queue.totalRecords}):
            </div>
            <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
              {service.stats.radarr.queue.records
                .slice(0, 3)
                .map(
                  (
                    record: RadarrQueueItem,
                    index: number,
                    array: RadarrQueueItem[]
                  ) => (
                    <div
                      key={record.id}
                      className={`flex flex-col space-y-1 overflow-hidden ${
                        index !== array.length - 1
                          ? "border-b border-gray-750 pb-2"
                          : ""
                      }`}
                    >
                      <div className="text-xs opacity-75">
                        <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                          Release:{" "}
                        </span>
                        <span className="text-xs overflow-hidden">
                          {record.title}
                        </span>
                      </div>
                      <div className="text-xs opacity-75">
                        <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                          State:{" "}
                        </span>
                        {record.trackedDownloadState}
                      </div>
                      {record.indexer && (
                        <div className="text-xs opacity-75">
                          <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                            Indexer:{" "}
                          </span>
                          {record.indexer}
                        </div>
                      )}
                      {record.customFormatScore != null && (
                        <div className="text-xs opacity-75">
                          <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                            Custom Format Score:{" "}
                          </span>
                          {record.customFormatScore}
                        </div>
                      )}
                      <div className="text-xs opacity-75">
                        <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                          Client:{" "}
                        </span>
                        {record.downloadClient}
                      </div>
                    </div>
                  )
                )}
            </div>
          </div>
        )}
    </div>
  );
};
