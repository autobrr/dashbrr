/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ProwlarrIndexer } from "../../../types/service";
import { ProwlarrMessage } from "./ProwlarrMessage";

interface ProwlarrStatsProps {
  instanceId: string;
}

export const ProwlarrStats: React.FC<ProwlarrStatsProps> = ({ instanceId }) => {
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

  const stats = service.stats?.prowlarr?.stats;
  const indexers = service.stats?.prowlarr?.indexers;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <ProwlarrMessage status={service.status} message={service.message} />

      {/* Stats Display */}
      {stats && stats.grabCount + stats.failCount + stats.indexerCount > 0 && (
        <div>
          <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
            Statistics:
          </div>
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
            <div className="grid grid-cols-3 gap-4">
              <div className="flex flex-col">
                <span className="font-medium text-gray-600 dark:text-gray-300">
                  Grabs
                </span>
                <span className="text-sm">{stats.grabCount}</span>
              </div>
              <div className="flex flex-col">
                <span className="font-medium text-gray-600 dark:text-gray-300">
                  Failures
                </span>
                <span className="text-sm">{stats.failCount}</span>
              </div>
              <div className="flex flex-col">
                <span className="font-medium text-gray-600 dark:text-gray-300">
                  Indexers
                </span>
                <span className="text-sm">{stats.indexerCount}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Active Indexers Display */}
      {indexers && indexers.length > 0 && (
        <div>
          <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
            Active Indexers:
          </div>
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
            {indexers
              .filter((indexer: ProwlarrIndexer) => indexer.enable)
              .sort(
                (a: ProwlarrIndexer, b: ProwlarrIndexer) =>
                  a.priority - b.priority
              )
              .slice(0, 5)
              .map((indexer: ProwlarrIndexer) => (
                <div
                  key={indexer.id}
                  className="flex justify-between items-center"
                >
                  <span className="font-medium">{indexer.name}</span>
                  <div className="flex items-center space-x-2">
                    <span className="text-xs opacity-75">
                      Priority: {indexer.priority}
                    </span>
                  </div>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
};
