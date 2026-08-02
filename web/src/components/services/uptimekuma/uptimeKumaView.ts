/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { UptimeKumaMonitor } from "../../../types/service";

export const UPTIME_KUMA_MONITOR_ROW_LIMIT = 5;

export type UptimeKumaFilter =
  | "total"
  | "up"
  | "down"
  | "pending"
  | "maintenance";

export interface UptimeKumaMonitorView {
  title: string;
  totalCount: number;
  monitors: UptimeKumaMonitor[];
}

const parseUptimeKumaBaseURL = (
  baseURL: string | null | undefined
): URL | null => {
  const candidate = baseURL?.trim();
  if (!candidate) return null;

  try {
    const url = new URL(candidate);
    return url.protocol === "http:" || url.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
};

const buildUptimeKumaURL = (
  baseURL: string | null | undefined,
  path: string
): string | null => {
  const url = parseUptimeKumaBaseURL(baseURL);
  if (!url) return null;

  const basePath = url.pathname.replace(/\/+$/, "");
  url.pathname = `${basePath}${path}`;
  url.search = "";
  url.hash = "";
  return url.toString();
};

export const resolveUptimeKumaBaseURL = (
  accessURL: string | null | undefined,
  serviceURL: string | null | undefined
): string | null => {
  for (const candidate of [accessURL, serviceURL]) {
    if (parseUptimeKumaBaseURL(candidate)) return candidate?.trim() ?? null;
  }

  return null;
};

export const buildUptimeKumaDashboardURL = (
  baseURL: string | null | undefined
): string | null =>
  buildUptimeKumaURL(baseURL, "/dashboard");

export const buildUptimeKumaMonitorURL = (
  baseURL: string | null | undefined,
  monitorID: string
): string | null =>
  buildUptimeKumaURL(baseURL, `/dashboard/${encodeURIComponent(monitorID)}`);

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
    totalCount: filteredMonitors.length,
    monitors: sortedMonitors.slice(0, UPTIME_KUMA_MONITOR_ROW_LIMIT),
  };
};
