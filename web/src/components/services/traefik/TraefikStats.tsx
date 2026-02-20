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

const formatDuration = (seconds?: number): string => {
  if (seconds === undefined || Number.isNaN(seconds)) {
    return "n/a";
  }
  if (seconds <= 0) {
    return "expired";
  }
  const days = Math.floor(seconds / 86400);
  if (days >= 1) {
    return `${days}d`;
  }
  const hours = Math.floor(seconds / 3600);
  if (hours >= 1) {
    return `${hours}h`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${Math.max(minutes, 1)}m`;
};

const formatTimestamp = (value?: string): string => {
  if (!value) {
    return "n/a";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
};

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

const certificateBadgeClass = (status: string): string => {
  if (status === "expired") {
    return "bg-rose-500/15 text-rose-300";
  }
  if (status === "expiring") {
    return "bg-amber-500/15 text-amber-300";
  }
  return "bg-emerald-500/15 text-emerald-300";
};

export const TraefikStats: React.FC<TraefikStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "traefik:issue_routers"),
    true
  );
  const { isExpanded: certsExpanded, toggle: toggleCerts } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "traefik:certificates"),
    false
  );

  const summary = service?.stats?.traefik?.summary;
  const details = service?.details?.traefik;
  const issueRouters = summary?.issueRouters ?? [];
  const certSummary = summary?.certificates;
  const certificates = certSummary?.certificates ?? [];

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
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Certs:{" "}
              <span className="font-semibold">
                {details.certificateTotal || 0}
              </span>{" "}
              ·{" "}
              <span className="font-semibold text-amber-300">
                {details.certificateExpiringSoon || 0} expiring
              </span>{" "}
              ·{" "}
              <span className="font-semibold text-rose-300">
                {details.certificateExpired || 0} expired
              </span>
            </span>
            <span className="rounded-md bg-zinc-900/80 px-2 py-1">
              Next expiry:{" "}
              <span className="font-semibold">
                {formatDuration(details.certificateNextExpiryInSeconds)}
              </span>{" "}
              ({formatTimestamp(details.certificateNextExpiry)})
            </span>
          </div>
        </>
      )}

      {certificates.length > 0 && (
        <CollapsibleSection
          title="Certificates"
          meta={`${certSummary?.total ?? certificates.length}`}
          isExpanded={certsExpanded}
          onToggle={toggleCerts}
        >
          <div className="space-y-2">
            {certificates.slice(0, 10).map((cert, index) => (
              <div
                key={`${cert.serial || cert.commonName || "cert"}:${cert.notAfterUnix}:${index}`}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium text-zinc-200">
                    {cert.commonName || cert.sans?.[0] || "Unnamed certificate"}
                  </span>
                  <span
                    className={`rounded px-1.5 py-0.5 text-[10px] uppercase tracking-wide ${certificateBadgeClass(cert.status)}`}
                  >
                    {cert.status}
                  </span>
                </div>
                <div className="mt-1 text-zinc-400">
                  Expires {formatDuration(cert.expiresInSeconds)} · {formatTimestamp(cert.notAfter)}
                </div>
              </div>
            ))}
          </div>
        </CollapsibleSection>
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
