/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ServiceStats } from "../../../types/service";
import { ArrMessage } from "./ArrMessage";
import { ArrQueueRecord, ArrQueueStatsBase } from "./ArrQueueStatsBase";

type ArrQueueServiceType = "sonarr" | "radarr" | "lidarr" | "readarr";

interface ArrQueueStatsProps {
  instanceId: string;
  serviceType: ArrQueueServiceType;
}

type ArrQueueStatsConfig = {
  serviceName: "Sonarr" | "Radarr" | "Lidarr" | "Readarr";
  queuePath:
    | "/api/sonarr/queue"
    | "/api/radarr/queue"
    | "/api/lidarr/queue"
    | "/api/readarr/queue";
  getQueue: (
    stats: ServiceStats
  ) => { totalRecords: number; records: ArrQueueRecord[] } | undefined;
  canManageRecord: (record: ArrQueueRecord) => boolean;
  getManageDisabledReason: (record: ArrQueueRecord) => string;
};

const canManageBlockedOrPending = (record: ArrQueueRecord) =>
  record.trackedDownloadState === "importBlocked" ||
  record.trackedDownloadState === "importPending";

const ARR_QUEUE_STATS_CONFIG: Record<ArrQueueServiceType, ArrQueueStatsConfig> = {
  sonarr: {
    serviceName: "Sonarr",
    queuePath: "/api/sonarr/queue",
    getQueue: (stats) => stats.sonarr?.queue,
    canManageRecord: (record) => record.trackedDownloadState === "importBlocked",
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked",
  },
  radarr: {
    serviceName: "Radarr",
    queuePath: "/api/radarr/queue",
    getQueue: (stats) => stats.radarr?.queue,
    canManageRecord: canManageBlockedOrPending,
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked or pending",
  },
  lidarr: {
    serviceName: "Lidarr",
    queuePath: "/api/lidarr/queue",
    getQueue: (stats) => stats.lidarr?.queue,
    canManageRecord: canManageBlockedOrPending,
    getManageDisabledReason: () =>
      "Can only remove items that are import blocked or pending",
  },
  readarr: {
    serviceName: "Readarr",
    queuePath: "/api/readarr/queue",
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
      queuePath={config.queuePath}
      getQueue={config.getQueue}
      canManageRecord={config.canManageRecord}
      getManageDisabledReason={config.getManageDisabledReason}
      renderMessage={({ status, message }) => (
        <ArrMessage status={status} message={message} />
      )}
    />
  );
};
