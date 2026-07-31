/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrMessage } from "../common/ArrMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface GeneralStatsProps {
  instanceId: string;
}

const MAX_FIELDS = 12;

const prettifyKey = (key: string): string =>
  key
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());

export const GeneralStats: React.FC<GeneralStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";

  if (isLoading) {
    return <StatsSkeleton rows={1} showRight={false} />;
  }

  if (!service) {
    return null;
  }

  // Only show message component if there's a message or status isn't online
  const showMessage = service.message || service.status !== "online";

  const fields = Object.entries(service.details?.general ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(0, MAX_FIELDS);

  return (
    <div className="space-y-4">
      {showMessage && (
        <ArrMessage status={service.status} message={service.message} />
      )}
      {fields.length > 0 && (
        <div className="text-xs rounded-md bg-gray-850/95 p-3.5">
          {fields.map(([key, value]) => (
            <div key={key} className="flex justify-between gap-4 py-0.5">
              <span className="text-gray-600 dark:text-gray-400">{prettifyKey(key)}</span>
              <span className="text-gray-700 dark:text-gray-200 truncate">{String(value)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
