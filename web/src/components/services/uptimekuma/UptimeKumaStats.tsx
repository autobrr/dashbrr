/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { useServiceData } from "../../../hooks/useServiceData";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { ArrMessage } from "../common/ArrMessage";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import type { UptimeKumaSummary } from "../../../types/service";

interface UptimeKumaStatsProps {
  instanceId: string;
}

const EMPTY_SUMMARY: UptimeKumaSummary = {
  monitors: [],
};

const countByStatus = (summary: UptimeKumaSummary, status: string): number =>
  summary.monitors.filter((monitor) => monitor.status === status).length;

const humanizeStatus = (status: string): string => {
  switch (status) {
    case "up":
      return "Up";
    case "down":
      return "Down";
    case "pending":
      return "Pending";
    case "maintenance":
      return "Maintenance";
    default:
      return "Unknown";
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

export const UptimeKumaStats: React.FC<UptimeKumaStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.uptimekuma?.summary ?? EMPTY_SUMMARY;

  const { isExpanded: issuesExpanded, toggle: toggleIssues } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "uptimekuma:issues"),
      true
    );

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

  const issueMonitors = summary.monitors.filter(
    (monitor) => monitor.status === "down" || monitor.status === "pending"
  );

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-5">
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Total</div>
          <div className="mt-0.5 text-zinc-100">{total}</div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Up</div>
          <div className="mt-0.5 text-emerald-300">{up}</div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Down</div>
          <div className="mt-0.5 text-red-300">{down}</div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Pending</div>
          <div className="mt-0.5 text-amber-300">{pending}</div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs col-span-2 sm:col-span-1">
          <div className="text-zinc-400">Maintenance</div>
          <div className="mt-0.5 text-blue-300">{maintenance}</div>
        </div>
      </div>

      {issueMonitors.length > 0 && (
        <CollapsibleSection
          title="Monitors With Issues"
          meta={`${issueMonitors.length}`}
          isExpanded={issuesExpanded}
          onToggle={toggleIssues}
        >
          <div className="space-y-1.5">
            {issueMonitors.slice(0, 10).map((monitor) => (
              <div
                key={monitor.id}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="truncate font-medium text-zinc-200">
                    {monitor.name}
                  </div>
                  <span
                    className={`inline-flex rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${statusBadgeClass(
                      monitor.status
                    )}`}
                  >
                    {humanizeStatus(monitor.status)}
                  </span>
                </div>
                <div className="mt-1 flex items-center justify-between gap-2 text-zinc-400">
                  <span className="truncate">{monitor.type || "monitor"}</span>
                  {typeof monitor.responseTimeMs === "number" &&
                    monitor.responseTimeMs > 0 && (
                    <span>{monitor.responseTimeMs}ms</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
