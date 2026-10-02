/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { UptimeKumaMonitor } from "../../../types/service";

export type UptimeKumaFilter =
  | "total"
  | "up"
  | "down"
  | "pending"
  | "maintenance";

export interface UptimeKumaMonitorView {
  title: string;
  monitors: UptimeKumaMonitor[];
}

const issuePriority = (monitor: UptimeKumaMonitor): number =>
  monitor.status === "down" ? 0 : 1;

export const getUptimeKumaMonitorView = (
  monitors: UptimeKumaMonitor[],
  filter: UptimeKumaFilter | null
): UptimeKumaMonitorView => {
  const filteredMonitors = filter === null
    ? monitors.filter(
      (monitor) => monitor.status === "down" || monitor.status === "pending"
    )
    : filter === "total"
      ? monitors
      : monitors.filter((monitor) => monitor.status === filter);

  const sortedMonitors = filter === null
    ? [...filteredMonitors].sort((left, right) => {
      const priorityDifference = issuePriority(left) - issuePriority(right);
      if (priorityDifference !== 0) return priorityDifference;
      return left.name.localeCompare(right.name, undefined, {
        sensitivity: "base",
      });
    })
    : filteredMonitors;

  const title = filter === null
    ? "Needs Attention"
    : filter === "total"
      ? "All Monitors"
      : `${filter[0].toUpperCase()}${filter.slice(1)} Monitors`;

  return {
    title,
    monitors: sortedMonitors,
  };
};
