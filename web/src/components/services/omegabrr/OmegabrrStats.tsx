/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { OmegabrrMessage } from "./OmegabrrMessage";
import { OmegabrrControls } from "./OmegabrrControls";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface OmegabrrStatsProps {
  instanceId: string;
}

export const OmegabrrStats: React.FC<OmegabrrStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  // Only show message component if there's a message or status isn't online
  const showMessage = service.message || service.status !== "online";

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      {showMessage && (
        <OmegabrrMessage status={service.status} message={service.message} />
      )}

      {/* Controls */}
      <OmegabrrControls instanceId={service.instanceId} />
    </div>
  );
};
