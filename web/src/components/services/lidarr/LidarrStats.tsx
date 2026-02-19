/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ArrMessage } from "../common/ArrMessage";
import { ArrQueueStatsBase } from "../common/ArrQueueStatsBase";

interface LidarrStatsProps {
  instanceId: string;
}

export const LidarrStats: React.FC<LidarrStatsProps> = ({ instanceId }) => {
  return (
    <ArrQueueStatsBase
      instanceId={instanceId}
      serviceName="Lidarr"
      queuePath="/api/lidarr/queue"
      getQueue={(stats) => stats.lidarr?.queue}
      canManageRecord={(record) =>
        record.trackedDownloadState === "importBlocked" ||
        record.trackedDownloadState === "importPending"
      }
      getManageDisabledReason={() =>
        "Can only remove items that are import blocked or pending"
      }
      renderMessage={({ status, message }) => (
        <ArrMessage status={status} message={message} />
      )}
    />
  );
};
