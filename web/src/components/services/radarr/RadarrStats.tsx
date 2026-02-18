/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ArrMessage } from "../common/ArrMessage";
import { ArrQueueStatsBase } from "../common/ArrQueueStatsBase";

interface RadarrStatsProps {
  instanceId: string;
}

export const RadarrStats: React.FC<RadarrStatsProps> = ({ instanceId }) => {
  return (
    <ArrQueueStatsBase
      instanceId={instanceId}
      serviceKey="radarr"
      serviceName="Radarr"
      queuePath="/api/radarr/queue"
      getQueue={(stats) => stats.radarr?.queue}
      canManageRecord={(record) =>
        record.trackedDownloadState === "importBlocked" ||
        record.trackedDownloadState === "importPending"
      }
      getManageDisabledReason={() =>
        "Can only remove items that are import blocked or pending"
      }
      renderMessage={({ status, message }) => (
        <ArrMessage status={status as never} message={message} />
      )}
    />
  );
};
