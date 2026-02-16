/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useState, useMemo } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ProwlarrIndexer } from "../../../types/service";
import { ProwlarrMessage } from "./ProwlarrMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import {
  ClockIcon,
  ArrowDownTrayIcon,
  MagnifyingGlassIcon,
  ChevronUpIcon,
} from "@heroicons/react/24/outline";

interface ProwlarrStatsProps {
  instanceId: string;
}

export const ProwlarrStats: React.FC<ProwlarrStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const [hasInitiallyLoaded, setHasInitiallyLoaded] = useState(false);
  const [stableIndexers, setStableIndexers] = useState<ProwlarrIndexer[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);

  const service = services.find((s) => s.instanceId === instanceId);
  const prowlarrData = service?.stats?.prowlarr;

  // Only show loading on initial load
  const isInitialLoading =
    !hasInitiallyLoaded &&
    (!prowlarrData?.indexers || prowlarrData.indexers.length === 0);

  // Memoize the filtered and sorted indexers to prevent unnecessary re-renders
  const activeIndexers = useMemo(() => {
    return stableIndexers
      .filter((indexer: ProwlarrIndexer) => indexer.enable)
      .sort((a: ProwlarrIndexer, b: ProwlarrIndexer) => a.priority - b.priority)
      .slice(0, 10);
  }, [stableIndexers]);

  // Update stable indexers only when we have complete data
  useEffect(() => {
    if (prowlarrData?.indexers?.length && prowlarrData.stats) {
      setStableIndexers(prowlarrData.indexers);
      if (!hasInitiallyLoaded) {
        setHasInitiallyLoaded(true);
      }
    }
  }, [prowlarrData?.indexers, prowlarrData?.stats, hasInitiallyLoaded]);

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service || !stableIndexers.length) {
    return null;
  }

  // Combine service message with health message if available
  const message = service.health?.message
    ? service.message
      ? `${service.message}\n${service.health.message}`
      : service.health.message
    : service.message;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <ProwlarrMessage status={service.status} message={message} />

      {/* Active Indexers */}
      <div>
        <div
          onClick={() => setIsExpanded(!isExpanded)}
          className="relative cursor-pointer select-none w-full flex items-center justify-between text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 group"
        >
          <span>Active Indexers <span className="text-xs font-bold lowercase">(Last 30 Days)</span></span>
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
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 pointer-events-none">
            <div className="divide-y divide-gray-850">
              {activeIndexers.map((indexer: ProwlarrIndexer) => (
                <div
                  key={indexer.id}
                  className="p-3 hover:bg-gray-800/50 transition-colors"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center min-w-0 gap-2">
                      <MagnifyingGlassIcon className="h-4 w-4 text-blue-400 flex-shrink-0" />
                      <span className="font-medium text-gray-200 truncate">
                        {indexer.name}
                      </span>
                      <div className="flex items-center gap-2 flex-shrink-0">
                        <span className="flex items-center gap-1 bg-gray-800/50 px-1.5 py-0.5 rounded">
                          <ArrowDownTrayIcon className="h-3.5 w-3.5" />
                          {indexer.numberOfGrabs || 0}
                        </span>
                        <span className="flex items-center gap-1 bg-gray-800/50 px-1.5 py-0.5 rounded">
                          <ClockIcon className="h-3.5 w-3.5" />
                          {indexer.averageResponseTime}ms
                        </span>
                      </div>
                    </div>
                    <span
                      title="Indexer Priority - Lower values have higher priority (1: Highest, 25: Default, 50: Lowest)"
                      className={`px-1.5 py-0.5 rounded flex-shrink-0 ${
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
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
