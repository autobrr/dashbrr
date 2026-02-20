/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

import { useServiceData } from "../../../hooks/useServiceData";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { ArrMessage } from "../common/ArrMessage";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import { CollapsibleSection } from "../../ui/CollapsibleSection";

interface TraefikStatsProps {
  instanceId: string;
}

const renderFeature = (value?: string): string =>
  value && value.trim() ? value : "Off";

const statusBadgeClass = (status: string): string => {
  const normalized = status.trim().toLowerCase();
  if (normalized === "warning") {
    return "bg-amber-500/15 text-amber-300";
  }
  if (normalized === "disabled") {
    return "bg-rose-500/15 text-rose-300";
  }
  return "bg-zinc-700/50 text-zinc-300";
};

export const TraefikStats: React.FC<TraefikStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "traefik:issue_routers"),
    true
  );

  const summary = service?.stats?.traefik?.summary;
  const details = service?.details?.traefik;
  const issueRouters = summary?.issueRouters ?? [];

  if (service?.status === "loading" && !summary) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      {details && (
        <>
          <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-3">
            <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
              <div className="text-zinc-400">Routers</div>
              <div className="mt-0.5 text-zinc-100">{details.routerTotal || 0}</div>
              <div className="text-zinc-400">
                {details.routerWarnings || 0} warnings · {details.routerErrors || 0} errors
              </div>
            </div>
            <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
              <div className="text-zinc-400">Services</div>
              <div className="mt-0.5 text-zinc-100">{details.serviceTotal || 0}</div>
              <div className="text-zinc-400">
                {details.serviceWarnings || 0} warnings · {details.serviceErrors || 0} errors
              </div>
            </div>
            <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
              <div className="text-zinc-400">Middlewares</div>
              <div className="mt-0.5 text-zinc-100">{details.middlewareTotal || 0}</div>
              <div className="text-zinc-400">
                {details.middlewareWarnings || 0} warnings · {details.middlewareErrors || 0} errors
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-2 text-xs text-zinc-300">
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Providers: <span className="font-semibold">{details.providers || 0}</span>
            </span>
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Metrics:{" "}
              <span className="font-semibold text-emerald-300">
                {renderFeature(details.metrics)}
              </span>
            </span>
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Tracing:{" "}
              <span className="font-semibold text-emerald-300">
                {renderFeature(details.tracing)}
              </span>
            </span>
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Access Log:{" "}
              <span className="font-semibold text-emerald-300">
                {details.accessLog ? "On" : "Off"}
              </span>
            </span>
          </div>
        </>
      )}

      {issueRouters.length > 0 && (
        <CollapsibleSection
          title="Issue Routers"
          meta={`${issueRouters.length}`}
          isExpanded={isExpanded}
          onToggle={toggle}
        >
          <div className="space-y-2">
            {issueRouters.slice(0, 10).map((router, index) => (
              <div
                key={`${router.name}:${router.provider}:${router.status}:${index}`}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium text-zinc-200">
                    {router.name || router.rule || "Unnamed router"}
                  </span>
                  <span
                    className={`rounded px-1.5 py-0.5 text-[10px] uppercase tracking-wide ${statusBadgeClass(router.status)}`}
                  >
                    {router.status || "unknown"}
                  </span>
                </div>
                <div className="mt-1 text-zinc-400">
                  {(router.provider || "unknown")} · {(router.rule || "no rule")}
                </div>
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
