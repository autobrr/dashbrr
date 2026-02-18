/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { ArrMessage } from "../common/ArrMessage";
import { ArrQueueStatsBase } from "../common/ArrQueueStatsBase";

interface SonarrStatsProps {
  instanceId: string;
}

export const SonarrStats: React.FC<SonarrStatsProps> = ({ instanceId }) => {
  return (
    <ArrQueueStatsBase
      instanceId={instanceId}
      serviceName="Sonarr"
      queuePath="/api/sonarr/queue"
      getQueue={(stats) => stats.sonarr?.queue}
      canManageRecord={(record) => record.trackedDownloadState === "importBlocked"}
      getManageDisabledReason={() =>
        "Can only remove items that are import blocked"
      }
      renderMessage={({ status, message }) => (
        <ArrMessage status={status} message={message} />
      )}
    />
  );
};
