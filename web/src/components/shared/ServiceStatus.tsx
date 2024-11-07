/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { ServiceStatus as ServiceStatusType } from "../../types/service";

interface ServiceStatusProps {
  status: ServiceStatusType;
  message?: string;
  className?: string;
}

export const ServiceStatus: React.FC<ServiceStatusProps> = ({
  status,
  message,
  className = "",
}) => {
  const statusConfig = {
    online: {
      color: "bg-green-500",
      text: "text-green-700 dark:text-green-300",
      label: "Online",
    },
    offline: {
      color: "bg-gray-500",
      text: "text-gray-700 dark:text-gray-300",
      label: "Offline",
    },
    error: {
      color: "bg-red-500",
      text: "text-red-700 dark:text-red-300",
      label: "Error",
    },
    warning: {
      color: "bg-yellow-500",
      text: "text-yellow-700 dark:text-yellow-300",
      label: "Warning",
    },
    loading: {
      color: "bg-blue-500",
      text: "text-blue-700 dark:text-blue-300",
      label: "Loading",
    },
    pending: {
      color: "bg-purple-500",
      text: "text-purple-700 dark:text-purple-300",
      label: "Not Configured",
    },
    unknown: {
      color: "bg-gray-500",
      text: "text-gray-700 dark:text-gray-300",
      label: "Unknown",
    },
  };

  const config = statusConfig[status] || statusConfig.unknown;

  return (
    <div className={`flex items-center space-x-2 ${className}`}>
      <div className={`w-2 h-2 rounded-full ${config.color}`} />
      <span className={`text-sm font-medium ${config.text}`}>
        {message || config.label}
      </span>
    </div>
  );
};
