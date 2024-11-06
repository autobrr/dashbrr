/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { RadarrQueueItem } from "../../../types/service";

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

  if (!service?.stats?.radarr?.queue) {
    return null;
  }

  const { queue } = service.stats.radarr;

  if (queue.totalRecords === 0) {
    return null;
  }

  // Group records by series name
  const groupedRecords = queue.records?.reduce<
    Record<string, RadarrQueueItem[]>
  >((acc, record) => {
    const seriesName = record.title?.split(".")[0] || "";
    if (!acc[seriesName]) {
      acc[seriesName] = [];
    }
    acc[seriesName].push(record);
    return acc;
  }, {});

  const uniqueSeries = Object.entries(groupedRecords || {}).slice(0, 3);

  return (
    <div className="mt-2 space-y-4">
      {queue.totalRecords > 0 && (
        <div>
          <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
            Queue ({queue.totalRecords}):
          </div>
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
            {uniqueSeries.map(([, records]) => {
              const firstRecord = records[0];
              return (
                <div
                  key={firstRecord.id}
                  className="flex flex-col space-y-1 overflow-hidden"
                >
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      Release:{" "}
                    </span>
                    {firstRecord.title}
                  </div>
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      Status:{" "}
                    </span>
                    {firstRecord.status}
                  </div>
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      State:{" "}
                    </span>
                    {firstRecord.trackedDownloadState}
                  </div>
                  {firstRecord.indexer != null && (
                    <div className="text-xs opacity-75">
                      <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                        Indexer:{" "}
                      </span>
                      {firstRecord.indexer}
                    </div>
                  )}
                  {firstRecord.customFormatScore && (
                    <div className="text-xs opacity-75">
                      <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                        Custom Format Score:{" "}
                      </span>
                      {firstRecord.customFormatScore}
                    </div>
                  )}
                  <div className="text-xs opacity-75">
                    <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                      Client:{" "}
                    </span>
                    {firstRecord.downloadClient}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};
