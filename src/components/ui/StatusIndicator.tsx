/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

type StatusType =
  | "online"
  | "offline"
  | "warning"
  | "error"
  | "loading"
  | "unknown"
  | "healthy"
  | "pending";

interface StatusIndicatorProps {
  status: StatusType;
  message?: string;
  isInitialLoad?: boolean;
  isConnected?: boolean;
}

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  status,
  message,
  isInitialLoad = false,
  isConnected = true,
}) => {
  const getMessageStyle = (status: StatusType) => {
    const baseStyles = "transition-all duration-200 backdrop-blur-sm";

    switch (status) {
      case "online":
        return `${baseStyles} text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30 border border-green-100 dark:border-green-900/50 shadow-sm shadow-green-100/50 dark:shadow-green-900/30`;
      case "warning":
        return `${baseStyles} text-yellow-600 dark:text-yellow-400 bg-yellow-50/90 dark:bg-yellow-900/30 border border-yellow-100 dark:border-yellow-900/50 shadow-sm shadow-yellow-100/50 dark:shadow-yellow-900/30`;
      case "offline":
      case "error":
        return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50 shadow-sm shadow-red-100/50 dark:shadow-red-900/30`;
      case "loading":
      case "pending":
        return `${baseStyles} text-blue-600 dark:text-blue-400 bg-blue-50/90 dark:bg-blue-900/30 border border-blue-100 dark:border-blue-900/50 shadow-sm shadow-blue-100/50 dark:shadow-blue-900/30`;
      default:
        return `${baseStyles} text-gray-600 dark:text-gray-400 bg-gray-50/90 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800 shadow-sm`;
    }
  };

  const getStatusDisplay = () => {
    if (isInitialLoad) {
      return {
        text: "Loading",
        color: "text-blue-500 dark:text-blue-400",
        icon: "⟳",
      };
    }

    if (!isConnected) {
      return {
        text: "Disconnected",
        color: "text-yellow-500 dark:text-yellow-400",
        icon: "⚠",
      };
    }

    switch (status) {
      case "online":
        return {
          text: "Healthy",
          color: "text-green-500 dark:text-green-400",
          icon: "✓",
        };
      case "loading":
        return {
          text: "Checking",
          color: "text-blue-500 dark:text-blue-400",
          icon: "⟳",
        };
      case "pending":
        return {
          text: "Pending",
          color: "text-blue-500 dark:text-blue-400",
          icon: "○",
        };
      case "warning":
        return {
          text: "Warning",
          color: "text-yellow-500 dark:text-yellow-400",
          icon: "⚠",
        };
      case "offline":
      case "error":
        return {
          text: "Error",
          color: "text-red-500 dark:text-red-400",
          icon: "⚠",
        };
      default:
        return {
          text: "Unknown",
          color: "text-gray-500 dark:text-gray-400",
          icon: "?",
        };
    }
  };

  const statusDisplay = getStatusDisplay();
  const shouldShowMessage = message && (status !== "online" || !isConnected);
  const displayMessage = isInitialLoad
    ? "Initializing service..."
    : !isConnected
    ? "Connection to server lost"
    : message;

  return (
    <div className="space-y-2 transition-all duration-200 mb-4">
      <div className="flex items-center gap-1.5 select-none">
        <span className="text-xs font-medium text-gray-700 dark:text-gray-100">
          Status
        </span>
        <div
          className={`flex items-center gap-1 ${statusDisplay.color} font-medium`}
        >
          <span className="text-xs pointer-events-none">
            {statusDisplay.text}
          </span>
          <span
            className={`text-xs ${
              status === "loading" || isInitialLoad ? "animate-spin" : ""
            }`}
          >
            {statusDisplay.icon}
          </span>
        </div>
      </div>

      {shouldShowMessage && (
        <div
          className={`flex items-center text-xs p-2 rounded-lg ${getMessageStyle(
            status
          )}`}
        >
          <div className="flex-1 min-h-[24px] flex items-center">
            {displayMessage}
            {(status === "loading" || isInitialLoad) && (
              <span className="inline-block animate-pulse ml-1">...</span>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
