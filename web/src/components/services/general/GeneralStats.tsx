/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { GeneralMessage } from "./GeneralMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface GeneralStatsProps {
  instanceId: string;
}

export const GeneralStats: React.FC<GeneralStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const isLoading = service?.status === "loading";

  if (isLoading) {
    return <StatsSkeleton rows={1} showRight={false} />;
  }

  if (!service) {
    return null;
  }

  // Only show message component if there's a message or status isn't online
  const showMessage = service.message || service.status !== "online";

  return (
    <div className="space-y-4">
      {showMessage && (
        <GeneralMessage status={service.status} message={service.message} />
      )}
    </div>
  );
};
