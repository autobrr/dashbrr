/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { ExclamationTriangleIcon } from "@heroicons/react/24/outline";

interface Props {
  message: string;
  type: "error" | "warning" | "offline";
}

export const MaintainerrMessage: React.FC<Props> = ({ message, type }) => {
  const isTemporaryError =
    message.includes("temporarily unavailable") ||
    message.includes("timed out") ||
    message.includes("Bad Gateway") ||
    message.includes("502") ||
    message.includes("503") ||
    message.includes("504");

  const getMessageStyle = () => {
    const baseStyles =
      "text-xs p-2 rounded-lg transition-all duration-200 backdrop-blur-sm";

    switch (type) {
      case "error":
        return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50`;
      case "warning":
        return `${baseStyles} text-amber-500 dark:text-amber-300 bg-amber-50/90 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40`;
      case "offline":
        return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50`;
      default:
        return `${baseStyles} text-gray-600 dark:text-gray-400 bg-gray-50/90 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800`;
    }
  };

  const getIconColor = () => {
    switch (type) {
      case "error":
      case "offline":
        return "text-red-500 dark:text-red-400";
      case "warning":
        return "text-amber-500 dark:text-amber-300";
      default:
        return "text-gray-500 dark:text-gray-400";
    }
  };

  const getMessage = () => {
    let finalMessage = message;

    if (type === "error" && isTemporaryError) {
      finalMessage += " - Please try again later";
    }

    if (type === "warning") {
      // Add warning-specific context if needed
      if (message.includes("version mismatch")) {
        finalMessage += " - Consider updating your installation";
      }
    }

    return finalMessage;
  };

  return (
    <div className={getMessageStyle()}>
      <div className="flex items-center space-x-2">
        <ExclamationTriangleIcon className={`w-4 h-4 ${getIconColor()}`} />
        <span>{getMessage()}</span>
      </div>
    </div>
  );
};
