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

    // Check for indexer warnings
    if (message.includes("Indexers unavailable")) {
      // Split the message into the warning and indexers part
      const [warningText, indexersText] = message.split(/:\s+/);
      // Get the indexers list before any movie filename
      const indexers = indexersText.split(/\s+\w+\./)[0].split(", ");

      return (
        <div className="space-y-2">
          <div className="opacity-90 font-bold">
            {warningText}:
            <ul className="list-disc font-normal pl-5 mt-1">
              {indexers.map((indexer, index) => (
                <li key={index}>{indexer.trim()}</li>
              ))}
            </ul>
          </div>
          {message.includes("Not an upgrade") ||
          message.includes("Not a Custom Format upgrade") ? (
            <div className="opacity-90 font-bold pt-2">
              Item(s) are stuck in import queue
            </div>
          ) : null}
        </div>
      );
    }

    // If only stuck downloads without indexer warning
    if (
      message.includes("Not an upgrade") ||
      message.includes("Not a Custom Format upgrade")
    ) {
      return <div className="opacity-90">Items stuck in import queue</div>;
    }

    // For any other messages, return as is
    return <div className="opacity-90">{message}</div>;
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
