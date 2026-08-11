/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { MaintainerrCollections } from "./MaintainerrCollections";
import { MaintainerrMessage } from "./MaintainerrMessage";
import {
  ExclamationTriangleIcon,
  CheckCircleIcon,
  XCircleIcon
} from "@heroicons/react/24/outline";
import { useTranslation } from "react-i18next";

interface MaintainerrServiceProps {
  instanceId: string;
}

export const MaintainerrService: React.FC<MaintainerrServiceProps> = ({
  instanceId,
}) => {
  const { t } = useTranslation();
  const { getService } = useServiceData();
  const service = getService(instanceId);

  const renderStatus = () => {
    if (!service) return null;

    const getStatusColor = () => {
      switch (service.status) {
        case "online":
          return "text-green-500 dark:text-green-400";
        case "error":
        case "offline":
          return "text-red-500 dark:text-red-400";
        case "warning":
          return "text-amber-500 dark:text-amber-300";
        case "loading":
          return "text-blue-500 dark:text-blue-400";
        default:
          return "text-gray-500 dark:text-gray-400";
      }
    };

    const getStatusIcon = () => {
      switch (service.status) {
        case "online":
          return <CheckCircleIcon className="w-4 h-4" />;
        case "error":
        case "offline":
          return <XCircleIcon className="w-4 h-4" />;
        case "warning":
          return <ExclamationTriangleIcon className="w-4 h-4" />;
        default:
          return null;
      }
    };

    const getStatusText = () => {
      switch (service.status) {
        case "online":
          return t("status.healthy", "Healthy");
        case "error":
          return t("status.error", "Error");
        case "offline":
          return t("status.offline", "Offline");
        case "warning":
          return t("status.warning", "Warning");
        case "loading":
          return t("status.loading", "Loading");
        default:
          return t("status.unknown", "Unknown");
      }
    };

    return (
      <div className="flex items-center gap-1.5 select-none pb-2">
        <span className="text-xs font-medium text-gray-700 dark:text-gray-100">
          {t("common.state", "Status")}
        </span>
        <div className={`flex items-center gap-1 ${getStatusColor()}`}>
          <span className="text-xs pointer-events-none">{getStatusText()}</span>
          {getStatusIcon()}
          {service.status === "loading" && (
            <span className="animate-spin text-xs">⟳</span>
          )}
        </div>
      </div>
    );
  };

  const renderMessages = () => {
    if (!service) return null;

    const messages = [];
    const error = service.status === "error" ? service.message : null;
    const warning = service.status === "warning" ? service.message : null;
    const isOffline = service.status === "offline";

    if (error) {
      messages.push(
        <MaintainerrMessage key="error" message={error} type="error" />
      );
    }

    if (warning) {
      messages.push(
        <MaintainerrMessage key="warning" message={warning} type="warning" />
      );
    }

    if (isOffline) {
      messages.push(
        <MaintainerrMessage
          key="offline"
          message={t("status.offline", "Service is offline")}
          type="offline"
        />
      );
    }

    return messages.length > 0 ? (
      <div className="space-y-2 mb-4">{messages}</div>
    ) : null;
  };

  return (
    <div className="space-y-2 mt-2">
      {renderStatus()}
      {renderMessages()}
      <MaintainerrCollections instanceId={instanceId} />
    </div>
  );
};
