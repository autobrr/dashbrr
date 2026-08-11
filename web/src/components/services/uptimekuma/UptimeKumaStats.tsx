/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { ArrowTopRightOnSquareIcon } from "@heroicons/react/20/solid";

import { useServiceData } from "../../../hooks/useServiceData";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { ArrMessage } from "../common/ArrMessage";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import type { UptimeKumaSummary } from "../../../types/service";
import {
  buildUptimeKumaDashboardURL,
  buildUptimeKumaMonitorURL,
  getUptimeKumaMonitorView,
  resolveUptimeKumaBaseURL,
  type UptimeKumaFilter
} from "./uptimeKumaView";
import { useTranslation } from "react-i18next";

interface UptimeKumaStatsProps {
  instanceId: string;
}

interface UptimeKumaCounts {
  total: number;
  up: number;
  down: number;
  pending: number;
  maintenance: number;
}

interface UptimeKumaStatsViewProps {
  baseURL: string | null;
  counts: UptimeKumaCounts;
  summary: UptimeKumaSummary;
}

const useFilterTiles = () => {
  const { t } = useTranslation();
  return [
    { filter: "total" as UptimeKumaFilter, label: t("uptimekuma.total", "Total"), valueClass: "text-zinc-100" },
    { filter: "up" as UptimeKumaFilter, label: t("uptimekuma.up", "Up"), valueClass: "text-emerald-300" },
    { filter: "down" as UptimeKumaFilter, label: t("uptimekuma.down", "Down"), valueClass: "text-red-300" },
    { filter: "pending" as UptimeKumaFilter, label: t("uptimekuma.pending", "Pending"), valueClass: "text-amber-300" },
    {
      filter: "maintenance" as UptimeKumaFilter,
      label: t("uptimekuma.maintenance", "Maintenance"),
      valueClass: "text-blue-300",
      layoutClass: "col-span-2 @md:col-span-1",
    },
  ];
};

const EMPTY_SUMMARY: UptimeKumaSummary = {
  monitors: [],
};

const countByStatus = (summary: UptimeKumaSummary, status: string): number =>
  summary.monitors.filter((monitor) => monitor.status === status).length;

const humanizeStatus = (status: string, t: any): string => {
  switch (status) {
    case "up":
      return t("uptimekuma.up", "Up");
    case "down":
      return t("uptimekuma.down", "Down");
    case "pending":
      return t("uptimekuma.pending", "Pending");
    case "maintenance":
      return t("uptimekuma.maintenance", "Maintenance");
    default:
      return t("status.unknown", "Unknown");
  }
};

const statusBadgeClass = (status: string): string => {
  switch (status) {
    case "up":
      return "bg-emerald-900/30 text-emerald-300";
    case "down":
      return "bg-red-900/30 text-red-300";
    case "pending":
      return "bg-amber-900/30 text-amber-300";
    case "maintenance":
      return "bg-blue-900/30 text-blue-300";
    default:
      return "bg-zinc-800 text-zinc-300";
  }
};

export const UptimeKumaStatsView: React.FC<UptimeKumaStatsViewProps> = ({
  baseURL,
  counts,
  summary,
}) => {
  const { t } = useTranslation();
  const FILTER_TILES = useFilterTiles();
  const monitorViewTitleID = React.useId();
  const [selectedFilter, setSelectedFilter] = React.useState<UptimeKumaFilter | null>(
    null
  );
  const activeFilter = selectedFilter && counts[selectedFilter] > 0
    ? selectedFilter
    : null;
  const monitorView = getUptimeKumaMonitorView(summary.monitors, activeFilter);
  const dashboardURL = buildUptimeKumaDashboardURL(baseURL);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-1.5 @md:grid-cols-5">
        {FILTER_TILES.map(({ filter, label, valueClass, layoutClass = "" }) => {
          const count = counts[filter];
          const isActive = activeFilter === filter;

          return (
            <button
              key={filter}
              type="button"
              disabled={count === 0}
              aria-label={`${label} ${count}`}
              aria-pressed={isActive}
              onClick={() =>
                setSelectedFilter((currentFilter) =>
                  currentFilter === filter ? null : filter
                )
              }
              className={`rounded-md bg-zinc-900/80 px-3.5 py-2 text-left text-xs outline-none transition-[background-color,box-shadow,transform] duration-150 enabled:hover:bg-zinc-900 enabled:active:translate-y-px enabled:focus-visible:ring-2 enabled:focus-visible:ring-blue-500/70 disabled:cursor-default disabled:opacity-60 ${
                isActive ? "bg-zinc-900 ring-1 ring-inset ring-zinc-500/80" : ""
              } ${layoutClass}`}
            >
              <div className="text-zinc-400">{label}</div>
              <div className={`mt-0.5 ${valueClass}`}>{count}</div>
            </button>
          );
        })}
      </div>

      {monitorView.monitors.length > 0 && (
        <section aria-labelledby={monitorViewTitleID}>
          <div className="mb-1.5 flex items-center justify-between gap-3">
            <h4
              id={monitorViewTitleID}
              className="text-xs font-semibold text-zinc-200"
            >
              {monitorView.title}
            </h4>
            <div className="flex shrink-0 items-center gap-1.5 text-[11px] text-zinc-400">
              <span>{t("uptimekuma.monitors", { count: monitorView.monitors.length, defaultValue: `${monitorView.monitors.length} monitors` })}</span>
              {dashboardURL && (
                <>
                  <span aria-hidden="true">·</span>
                  <a
                    href={dashboardURL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-blue-400 transition-colors hover:text-blue-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/70"
                  >
                    {t("uptimekuma.open", "Open Uptime Kuma")}
                  </a>
                </>
              )}
            </div>
          </div>
          <div className="max-h-80 overflow-y-auto scrollbar-small divide-y divide-zinc-700/60 border-y border-zinc-700/60">
            {monitorView.monitors.map((monitor) => {
              const monitorURL = buildUptimeKumaMonitorURL(baseURL, monitor.id);
              const rowClassName =
                "flex items-center justify-between gap-3 px-1 py-2.5 text-xs";
              const rowContent = (
                <>
                  <div className="min-w-0">
                    <div className="truncate font-medium text-zinc-200 transition-colors group-hover/monitor:text-blue-300">
                      {monitor.name}
                    </div>
                    <div className="mt-0.5 flex items-center gap-2 text-zinc-400">
                      <span className="truncate">{monitor.type || t("uptimekuma.monitor", "monitor")}</span>
                      {typeof monitor.responseTimeMs === "number" &&
                      monitor.responseTimeMs > 0 && (
                        <span>{monitor.responseTimeMs}ms</span>
                      )}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <span
                      className={`inline-flex rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${statusBadgeClass(
                        monitor.status
                      )}`}
                    >
                      {humanizeStatus(monitor.status, t)}
                    </span>
                    {monitorURL && (
                      <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover/monitor:text-blue-400" />
                    )}
                  </div>
                </>
              );

              if (!monitorURL) {
                return (
                  <div key={monitor.id} className={rowClassName}>
                    {rowContent}
                  </div>
                );
              }

              return (
                <a
                  key={monitor.id}
                  href={monitorURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={`Open ${monitor.name}, ${monitor.type || t("uptimekuma.monitor", "monitor")}, ${humanizeStatus(
                    monitor.status,
                    t
                  )} in Uptime Kuma`}
                  className={`group/monitor ${rowClassName} transition-colors hover:bg-zinc-900/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/70`}
                >
                  {rowContent}
                </a>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
};

export const UptimeKumaStats: React.FC<UptimeKumaStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.uptimekuma?.summary ?? EMPTY_SUMMARY;

  const isInitialLoading =
    service?.status === "loading" && !service?.stats?.uptimekuma?.summary;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);
  const total = service.details?.uptimekuma?.total ?? summary.monitors.length;
  const up = service.details?.uptimekuma?.up ?? countByStatus(summary, "up");
  const down = service.details?.uptimekuma?.down ?? countByStatus(summary, "down");
  const pending =
    service.details?.uptimekuma?.pending ?? countByStatus(summary, "pending");
  const maintenance =
    service.details?.uptimekuma?.maintenance ??
    countByStatus(summary, "maintenance");
  const baseURL = resolveUptimeKumaBaseURL(service.accessUrl, service.url);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />
      <UptimeKumaStatsView
        baseURL={baseURL}
        counts={{ total, up, down, pending, maintenance }}
        summary={summary}
      />
    </div>
  );
};
