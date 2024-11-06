/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { LoadingState } from "./LoadingState";

export type HealthStatus = "healthy" | "unhealthy" | "unknown" | "checking";

interface HealthIndicatorProps {
  status: HealthStatus;
  lastChecked?: Date;
  message?: string;
  className?: string;
}

export const HealthIndicator: React.FC<HealthIndicatorProps> = ({
  status,
  lastChecked,
  message,
  className = "",
}) => {
  const statusConfig = {
    healthy: {
      color: "bg-green-500",
      icon: "✓",
      text: "Healthy",
    },
    unhealthy: {
      color: "bg-red-500 animate-pulse",
      icon: "✕",
      text: "Unhealthy",
    },
    unknown: {
      color: "bg-gray-500",
      icon: "?",
      text: "Unknown",
    },
    checking: {
      color: "bg-blue-500",
      icon: null,
      text: "Checking",
    },
  };

  const config = statusConfig[status];

  if (status === "checking") {
    return <LoadingState size="sm" message="Checking health..." />;
  }

  return (
    <div className={`flex items-center space-x-3 ${className}`}>
      <div
        className={`w-3 h-3 rounded-full ${config.color} flex items-center justify-center text-white text-xs`}
      >
        {config.icon}
      </div>
      <div className="flex flex-col">
        <span className="text-sm font-medium text-gray-900 dark:text-white">
          {message || config.text}
        </span>
        {lastChecked && (
          <span className="text-xs text-gray-500 dark:text-gray-400">
            Last checked: {lastChecked.toLocaleTimeString()}
          </span>
        )}
      </div>
    </div>
  );
};
