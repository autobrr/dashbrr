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
  | "unknown";

interface StatusIconProps {
  status: StatusType;
}

export const StatusIcon: React.FC<StatusIconProps> = ({ status }) => {
  const getStatusStyles = (status: StatusType) => {
    const baseStyles = "w-2.5 h-2.5 rounded-full transition-all duration-200";
    const pulseAnimation =
      "after:animate-ping after:absolute after:inset-0 after:rounded-full";
    const glowEffect =
      "before:absolute before:inset-[-4px] before:rounded-full before:bg-current before:opacity-20 before:blur-sm";

    switch (status) {
      case "online":
        return `${baseStyles} ${glowEffect} relative bg-green-500 after:bg-green-500/50 text-green-500`;
      case "offline":
        return `${baseStyles} ${glowEffect} relative bg-red-500 text-red-500`;
      case "error":
        return `${baseStyles} ${glowEffect} ${pulseAnimation} relative bg-red-500 after:bg-red-500/50 text-red-500`;
      case "warning":
        return `${baseStyles} ${glowEffect} ${pulseAnimation} relative bg-yellow-500 after:bg-yellow-500/50 text-yellow-500`;
      case "loading":
        return `${baseStyles} ${glowEffect} ${pulseAnimation} relative bg-blue-500 after:bg-blue-500/50 text-blue-500`;
      default:
        return `${baseStyles} ${glowEffect} relative bg-gray-500 text-gray-500`;
    }
  };

  const getTooltipText = (status: StatusType) => {
    switch (status) {
      case "online":
        return "Service is online";
      case "offline":
        return "Service is offline";
      case "error":
        return "Service error";
      case "warning":
        return "Service warning";
      case "loading":
        return "Loading status...";
      default:
        return "Status unknown";
    }
  };

  return (
    <div className="flex items-center space-x-1">
      <div className="relative" title={getTooltipText(status)}>
        <div className={getStatusStyles(status)} />
        {status === "loading" && (
          <div className="absolute inset-0 animate-spin">
            <div className="w-full h-full rounded-full border-2 border-transparent border-t-blue-500/30" />
          </div>
        )}
      </div>
    </div>
  );
};
