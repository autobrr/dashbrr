/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import {
  ExclamationTriangleIcon,
  CheckCircleIcon,
} from "@heroicons/react/24/outline";

interface Props {
  message?: string;
  status:
    | "online"
    | "offline"
    | "warning"
    | "error"
    | "loading"
    | "unknown"
    | "healthy"
    | "pending";
}

export const RadarrMessage: React.FC<Props> = ({ message, status }) => {
  const getMessageStyle = () => {
    const baseStyles =
      "text-xs p-2 rounded-lg transition-all duration-200 backdrop-blur-sm";

    switch (status) {
      case "error":
      case "offline":
        return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50`;
      case "warning":
        return `${baseStyles} text-amber-500 dark:text-amber-300 bg-amber-50/90 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40`;
      case "online":
      case "healthy":
        return `${baseStyles} text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30 border border-green-100 dark:border-green-900/50`;
      case "loading":
      case "pending":
        return `${baseStyles} text-blue-600 dark:text-blue-400 bg-blue-50/90 dark:bg-blue-900/30 border border-blue-100 dark:border-blue-900/50`;
      default:
        return `${baseStyles} text-gray-600 dark:text-gray-400 bg-gray-50/90 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800`;
    }
  };

  const getStatusDisplay = () => {
    switch (status) {
      case "online":
      case "healthy":
        return {
          text: "Healthy",
          icon: (
            <CheckCircleIcon className="w-4 h-4 text-green-500 dark:text-green-400" />
          ),
          color: "text-green-500 dark:text-green-400",
        };
      case "warning":
        return {
          text: "Warning",
          icon: (
            <ExclamationTriangleIcon className="w-4 h-4 text-amber-500 dark:text-amber-300" />
          ),
          color: "text-amber-500 dark:text-amber-300",
        };
      case "error":
      case "offline":
        return {
          text: "Error",
          icon: (
            <ExclamationTriangleIcon className="w-4 h-4 text-red-500 dark:text-red-400" />
          ),
          color: "text-red-500 dark:text-red-400",
        };
      case "loading":
      case "pending":
        return {
          text: "Loading",
          icon: (
            <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          ),
          color: "text-blue-500 dark:text-blue-400",
        };
      default:
        return {
          text: "Unknown",
          icon: (
            <ExclamationTriangleIcon className="w-4 h-4 text-gray-500 dark:text-gray-400" />
          ),
          color: "text-gray-500 dark:text-gray-400",
        };
    }
  };

  const formatMessage = () => {
    if (!message) return null;

    const warnings = message.split("\n\n");

    return (
      <div className="space-y-4">
        {/* Indexer warnings */}
        {warnings.some((w) => w.includes("Indexers unavailable")) && (
          <div className="space-y-2">
            <div className="opacity-90 font-bold">
              Indexers unavailable due to failures for more than 6 hours:
              <ul className="list-disc font-normal pl-5 mt-1">
                {warnings
                  .filter((w) => w.includes("Indexers unavailable"))
                  .flatMap((w) =>
                    w
                      .split(/:\s+/)[1]
                      .split(/\s+\w+\./)[0]
                      .split(", ")
                  )
                  .map((indexer, index) => (
                    <li key={index}>
                      {indexer.trim().replace(/\s*Wiki$/, "")}
                    </li>
                  ))}
              </ul>
            </div>
          </div>
        )}

        {/* Notification warnings */}
        {warnings.some((w) => w.includes("Notifications unavailable")) && (
          <div className="space-y-2">
            <div className="opacity-90 font-bold">
              Notifications unavailable due to failures:
              <ul className="list-disc font-normal pl-5 mt-1">
                {warnings
                  .filter((w) => w.includes("Notifications unavailable"))
                  .flatMap((w) => w.split(/:\s+/)[1].split(", "))
                  .map((notifier, index) => (
                    <li key={index}>{notifier.trim()}</li>
                  ))}
              </ul>
            </div>
          </div>
        )}

        {/* Queue warnings - only show once if any release has upgrade issues */}
        {warnings.some(
          (w) =>
            (w.includes(".WEBRip.") || w.includes("WEB-DL")) &&
            (w.includes("Not an upgrade") ||
              w.includes("Not a Custom Format upgrade"))
        ) && (
          <div className="opacity-90 font-bold">
            Item(s) are stuck in import queue
          </div>
        )}
      </div>
    );
  };

  const statusDisplay = getStatusDisplay();

  return (
    <div className="space-y-2">
      {/* Status Display */}
      <div className="flex items-center gap-1.5 select-none pb-2">
        <span className="text-xs font-medium text-gray-700 dark:text-gray-100">
          Status
        </span>
        <div className={`flex items-center gap-1 ${statusDisplay.color}`}>
          <span className="text-xs pointer-events-none">
            {statusDisplay.text}
          </span>
          {statusDisplay.icon}
        </div>
      </div>

      {/* Message Content */}
      {message && (
        <div className={getMessageStyle()}>
          <div className="flex items-start space-x-2">
            <div className="space-y-1">{formatMessage()}</div>
          </div>
        </div>
      )}
    </div>
  );
};
