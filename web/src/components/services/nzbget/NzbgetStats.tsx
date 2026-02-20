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
import type { NzbgetSummary } from "../../../types/service";

interface NzbgetStatsProps {
  instanceId: string;
}

const EMPTY_SUMMARY: NzbgetSummary = {
  status: {
    RemainingSizeLo: 0,
    RemainingSizeHi: 0,
    RemainingSizeMB: 0,
    DownloadRate: 0,
    DownloadRateLo: 0,
    DownloadRateHi: 0,
    DownloadPaused: false,
    PostPaused: false,
    ScanPaused: false,
    ServerStandBy: false,
    QuotaReached: false,
    ServerTime: 0,
    ResumeTime: 0,
    FreeDiskSpaceLo: 0,
    FreeDiskSpaceHi: 0,
    FreeDiskSpaceMB: 0,
    TotalDiskSpaceLo: 0,
    TotalDiskSpaceHi: 0,
    TotalDiskSpaceMB: 0,
  },
  queue: [],
  failedCount: 0,
  recentFailures: [],
};

const joinHiLo = (hi = 0, lo = 0): number => hi * 2 ** 32 + lo;

const formatBytes = (value: number): string => {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let current = value;
  let unit = 0;
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024;
    unit++;
  }
  return `${current.toFixed(current >= 100 ? 0 : 1)} ${units[unit]}`;
};

const formatSpeed = (value: number): string => `${formatBytes(value)}/s`;

const formatStatus = (summary: NzbgetSummary): string => {
  if (summary.status.DownloadPaused) return "Paused";
  if (summary.status.ServerStandBy) return "Standby";
  return "Running";
};

export const NzbgetStats: React.FC<NzbgetStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.nzbget?.summary ?? EMPTY_SUMMARY;

  const { isExpanded: queueExpanded, toggle: toggleQueue } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "nzbget:queue"),
      true
    );
  const { isExpanded: failuresExpanded, toggle: toggleFailures } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "nzbget:failures"),
      true
    );

  const isInitialLoading =
    service?.status === "loading" &&
    !service?.stats?.nzbget?.summary &&
    !service?.details?.nzbget;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);
  const remainingBytes = joinHiLo(
    summary.status.RemainingSizeHi,
    summary.status.RemainingSizeLo
  );
  const freeDiskBytes = joinHiLo(
    summary.status.FreeDiskSpaceHi,
    summary.status.FreeDiskSpaceLo
  );
  const speedBps = Math.max(
    joinHiLo(summary.status.DownloadRateHi, summary.status.DownloadRateLo),
    summary.status.DownloadRate
  );

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Queue Status</div>
          <div className="mt-0.5 text-zinc-100">{formatStatus(summary)}</div>
          <div className="text-zinc-400">
            {summary.queue.length} active
            {summary.failedCount > 0 ? ` · ${summary.failedCount} failure(s)` : ""}
          </div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Download Speed</div>
          <div className="mt-0.5 text-zinc-100">{formatSpeed(speedBps)}</div>
          <div className="text-zinc-400">
            Remaining {formatBytes(remainingBytes)}
          </div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Disk Free</div>
          <div className="mt-0.5 text-zinc-100">{formatBytes(freeDiskBytes)}</div>
          {summary.status.QuotaReached && (
            <div className="text-amber-300">Quota reached</div>
          )}
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Pause State</div>
          <div className="mt-0.5 text-zinc-100">
            {summary.status.DownloadPaused ? "Download paused" : "Download active"}
          </div>
          <div className="text-zinc-400">
            {summary.status.PostPaused ? "Post paused" : "Post active"} ·{" "}
            {summary.status.ScanPaused ? "Scan paused" : "Scan active"}
          </div>
        </div>
      </div>

      {summary.queue.length > 0 && (
        <CollapsibleSection
          title="Queue"
          meta={`${summary.queue.length}`}
          isExpanded={queueExpanded}
          onToggle={toggleQueue}
        >
          <div className="space-y-1.5">
            {summary.queue.slice(0, 10).map((item) => (
              <div
                key={item.NZBID}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="truncate font-medium text-zinc-200">
                  {item.NZBName || `Item ${item.NZBID}`}
                </div>
                <div className="mt-1 flex items-center justify-between text-zinc-400">
                  <span>{item.Status || "Unknown"}</span>
                  <span>{formatBytes(item.RemainingSizeMB * 1024 * 1024)} left</span>
                </div>
                {item.Category && (
                  <div className="mt-1 truncate text-zinc-500">{item.Category}</div>
                )}
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}

      {(summary.failedCount > 0 || summary.recentFailures.length > 0) && (
        <CollapsibleSection
          title="Recent Failures"
          meta={`${summary.failedCount}`}
          isExpanded={failuresExpanded}
          onToggle={toggleFailures}
        >
          <div className="space-y-1.5">
            {summary.recentFailures.length > 0 ? (
              summary.recentFailures.slice(0, 10).map((failure) => (
                <div
                  key={failure.NZBID}
                  className="rounded-md border border-amber-800/40 bg-amber-900/20 px-3.5 py-2 text-xs text-amber-300"
                >
                  <div className="truncate font-medium">
                    {failure.Name || failure.NZBName || `Item ${failure.NZBID}`}
                  </div>
                  <div className="mt-1 truncate text-amber-200">
                    {failure.Status || "FAILURE"}
                  </div>
                </div>
              ))
            ) : (
              <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs text-zinc-400">
                Failures detected, waiting for history details.
              </div>
            )}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};

