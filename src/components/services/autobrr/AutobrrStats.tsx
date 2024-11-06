/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { StatusIcon } from "../../ui/StatusIcon";

interface AutobrrStatsProps {
  instanceId: string;
}

export const AutobrrStats: React.FC<AutobrrStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);

  if (!service?.stats?.autobrr || !service?.details?.autobrr?.irc) {
    return null;
  }

  const stats = service.stats.autobrr;
  const ircStatus = service.details.autobrr.irc;

  return (
    <div className="mt-2 space-y-4">
      <div>
        <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
          IRC Status:
        </div>
        <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-1">
          {ircStatus.map((irc, index) => (
            <div key={index} className="flex justify-between items-center">
              <span className="font-medium">{irc.name}</span>
              <div className="flex items-center">
                <StatusIcon status={irc.healthy ? "online" : "error"} />
                <span className="ml-2" style={{ color: "inherit" }}>
                  {irc.healthy ? "Healthy" : "Unhealthy"}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div>
        <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
          Stats:
        </div>
        <div className="grid grid-cols-2 gap-2">
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium">Filtered Releases:</span>
            </div>
            <div className="font-bold">{stats.filtered_count || 0}</div>
          </div>

          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium">Approved Pushes:</span>
            </div>
            <div className="font-bold">{stats.push_approved_count}</div>
          </div>

          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium">Rejected Pushes:</span>
            </div>
            <div className="font-bold">{stats.push_rejected_count}</div>
          </div>

          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium">Errored Pushes:</span>
            </div>
            <div className="font-bold">{stats.push_error_count}</div>
          </div>
        </div>
      </div>
    </div>
  );
};

// Remove the separate AutobrrIRC component since it's now combined
export const AutobrrIRC = () => null;
