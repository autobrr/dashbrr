/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { PlexSessionDisplay } from "./PlexSessionDisplay";

interface PlexCardProps {
  instanceId: string;
}

export const PlexCard: React.FC<PlexCardProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  // Only show loading state when we have no sessions data at all
  const isLoading =
    service?.status === "loading" && !service?.stats?.plex?.sessions;

  return (
    <div className="mt-2 space-y-2">
      <PlexSessionDisplay
        sessions={service?.stats?.plex?.sessions || []}
        isLoading={isLoading}
      />
    </div>
  );
};
