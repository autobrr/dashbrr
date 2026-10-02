/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ArrQueue, ArrQueueItem, ServiceStats } from "../../../types/service";
import { ArrMessage } from "./ArrMessage";
import { ArrQueueStatsBase } from "./ArrQueueStatsBase";

type ArrQueueServiceType =
  | "sonarr"
  | "whisparr"
  | "radarr"
  | "lidarr"
  | "readarr";

interface ArrQueueStatsProps {
  instanceId: string;
  serviceType: ArrQueueServiceType;
}

type ArrQueueStatsConfig = {
  serviceName: "Sonarr" | "Whisparr" | "Radarr" | "Lidarr" | "Readarr";
  getQueue: (stats: ServiceStats) => ArrQueue | undefined;
  canManageRecord: (record: ArrQueueItem) => boolean;
  getManageDisabledReason: (record: ArrQueueItem) => string;
};

const canManageBlockedOrPending = (record: ArrQueueItem) =>
  record.trackedDownloadState === "importBlocked" ||
  record.trackedDownloadState === "importPending";

const ARR_QUEUE_STATS_CONFIG: Record<ArrQueueServiceType, ArrQueueStatsConfig> = {
  sonarr: {
    serviceName: "Sonarr",
    getQueue: (stats) => stats.sonarr?.queue,
    canManageRecord: (record) => record.trackedDownloadState === "importBlocked",
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked",
  },
  whisparr: {
    serviceName: "Whisparr",
    getQueue: (stats) => stats.whisparr?.queue,
    canManageRecord: (record) => record.trackedDownloadState === "importBlocked",
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked",
  },
  radarr: {
    serviceName: "Radarr",
    getQueue: (stats) => stats.radarr?.queue,
    canManageRecord: canManageBlockedOrPending,
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked or pending",
  },
  lidarr: {
    serviceName: "Lidarr",
    getQueue: (stats) => stats.lidarr?.queue,
    canManageRecord: canManageBlockedOrPending,
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked or pending",
  },
  readarr: {
    serviceName: "Readarr",
    getQueue: (stats) => stats.readarr?.queue,
    canManageRecord: canManageBlockedOrPending,
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked or pending",
  },
};

export const ArrQueueStats: React.FC<ArrQueueStatsProps> = ({
  instanceId,
  serviceType,
}) => {
  const config = ARR_QUEUE_STATS_CONFIG[serviceType];
  return (
    <ArrQueueStatsBase
      instanceId={instanceId}
      serviceName={config.serviceName}
      getQueue={config.getQueue}
      canManageRecord={config.canManageRecord}
      getManageDisabledReason={config.getManageDisabledReason}
      renderMessage={({ status, message }) => (
        <ArrMessage status={status} message={message} />
      )}
    />
  );
};
