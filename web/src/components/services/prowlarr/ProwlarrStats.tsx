/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ProwlarrIndexer } from "../../../types/service";
import { ProwlarrMessage } from "./ProwlarrMessage";
import {
  ClockIcon,
  ArrowDownTrayIcon,
  MagnifyingGlassIcon,
} from "@heroicons/react/24/solid";

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

  const indexers = service.stats?.prowlarr?.indexers;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <ProwlarrMessage status={service.status} message={service.message} />

      {/* Active Indexers Display */}
      {indexers && indexers.length > 0 && (
        <div>
          <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
            Active Indexers:
          </div>
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2.5">
            {indexers
              .filter((indexer: ProwlarrIndexer) => indexer.enable)
              .sort(
                (a: ProwlarrIndexer, b: ProwlarrIndexer) =>
                  a.priority - b.priority
              )
              .slice(0, 10)
              .map((indexer: ProwlarrIndexer) => (
                <div
                  key={indexer.id}
                  className="flex justify-between items-center cursor-pointer py-1.5 px-2 rounded-md hover:bg-gray-800/50 transition-colors"
                >
                  <div className="flex items-center space-x-2">
                    <span className="font-medium text-gray-300">
                      {indexer.name}
                    </span>
                    <span
                      title="Indexer Priority - Lower values have higher priority (1: Highest, 25: Default, 50: Lowest)"
                      className={`px-1.5 py-0.5 text-[10px] rounded-full ${
                        indexer.priority === 1
                          ? "bg-green-500/10 text-green-400/80"
                          : indexer.priority <= 3
                          ? "bg-blue-500/10 text-blue-400/80"
                          : indexer.priority <= 5
                          ? "bg-indigo-500/10 text-indigo-400/80"
                          : indexer.priority <= 7
                          ? "bg-purple-500/10 text-purple-400/80"
                          : indexer.priority <= 10
                          ? "bg-yellow-500/10 text-yellow-400/80"
                          : "bg-red-500/10 text-red-400/80"
                      }`}
                    >
                      P{indexer.priority}
                    </span>
                  </div>
                  <div className="flex items-center space-x-3 text-[10px]">
                    <div
                      title="Number of successful grabs"
                      className="flex items-center space-x-1"
                    >
                      <ArrowDownTrayIcon className="w-3 h-3 text-gray-500" />
                      <span className="text-gray-400">
                        {indexer.numberOfGrabs || 0}
                      </span>
                    </div>
                    {indexer.numberOfQueries > 0 && (
                      <div
                        title="Number of search queries"
                        className="flex items-center space-x-1"
                      >
                        <MagnifyingGlassIcon className="w-3 h-3 text-gray-500" />
                        <span className="text-gray-400">
                          {indexer.numberOfQueries}
                        </span>
                      </div>
                    )}
                    <div
                      title="Average response time in milliseconds"
                      className="flex items-center space-x-1"
                    >
                      <ClockIcon className="w-3 h-3 text-gray-500" />
                      <span className="text-gray-400">
                        {indexer.averageResponseTime}ms
                      </span>
                    </div>
                  </div>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
};
