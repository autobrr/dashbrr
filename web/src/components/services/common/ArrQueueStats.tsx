/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ArrQueue, ServiceStats } from "../../../types/service";
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
};

const ARR_QUEUE_STATS_CONFIG: Record<ArrQueueServiceType, ArrQueueStatsConfig> = {
  sonarr: {
    serviceName: "Sonarr",
    getQueue: (stats) => stats.sonarr?.queue,
  },
  whisparr: {
    serviceName: "Whisparr",
    getQueue: (stats) => stats.whisparr?.queue,
  },
  radarr: {
    serviceName: "Radarr",
    getQueue: (stats) => stats.radarr?.queue,
  },
  lidarr: {
    serviceName: "Lidarr",
    getQueue: (stats) => stats.lidarr?.queue,
  },
  readarr: {
    serviceName: "Readarr",
    getQueue: (stats) => stats.readarr?.queue,
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
      renderMessage={({ status, message }) => (
        <ArrMessage status={status} message={message} />
      )}
    />
  );
};
