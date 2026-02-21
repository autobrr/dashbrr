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
import type { SabnzbdSummary } from "../../../types/service";

interface SabnzbdStatsProps {
  instanceId: string;
}

const EMPTY_SUMMARY: SabnzbdSummary = {
  queue: {
    version: "",
    status: "",
    paused: false,
    speed: "",
    kbpersec: "",
    timeleft: "",
    sizeleft: "",
    size: "",
    mbleft: "",
    mb: "",
    noofslots: "0",
    noofslots_total: "0",
    diskspace1: "",
    diskspace2: "",
    diskspacetotal1: "",
    diskspacetotal2: "",
    diskspace1_norm: "",
    diskspace2_norm: "",
    have_warnings: "0",
    speedlimit_abs: "",
    slots: [],
  },
  failedCount: 0,
  recentFailures: [],
};

const parseCount = (raw: string): number => {
  const trimmed = raw.trim();
  if (!trimmed) return 0;
  const num = Number(trimmed);
  return Number.isFinite(num) ? num : 0;
};

const formatSpeed = (raw: string): string => {
  const speed = raw.trim();
  if (!speed) return "0 B/s";
  if (speed.endsWith("/s")) return speed;
  return `${speed}/s`;
};

export const SabnzbdStats: React.FC<SabnzbdStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const summary = service?.stats?.sabnzbd?.summary ?? EMPTY_SUMMARY;

  const { isExpanded: queueExpanded, toggle: toggleQueue } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "sabnzbd:queue"),
      true
    );
  const { isExpanded: failuresExpanded, toggle: toggleFailures } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "sabnzbd:failures"),
      true
    );

  const isInitialLoading =
    service?.status === "loading" &&
    !service?.stats?.sabnzbd?.summary &&
    !service?.details?.sabnzbd;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);
  const queueCount = parseCount(summary.queue.noofslots);
  const warningsCount = parseCount(summary.queue.have_warnings);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Queue Status</div>
          <div className="mt-0.5 text-zinc-100">
            {summary.queue.status || "Unknown"}
          </div>
          <div className="text-zinc-400">
            {queueCount} active
            {warningsCount > 0 ? ` · ${warningsCount} warning(s)` : ""}
          </div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Download Speed</div>
          <div className="mt-0.5 text-zinc-100">
            {formatSpeed(summary.queue.speed)}
          </div>
          <div className="text-zinc-400">
            Remaining {summary.queue.sizeleft || "0 B"} · ETA{" "}
            {summary.queue.timeleft || "0:00:00"}
          </div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Disk Free (incomplete)</div>
          <div className="mt-0.5 text-zinc-100">
            {summary.queue.diskspace1_norm || "Unknown"}
          </div>
        </div>
        <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
          <div className="text-zinc-400">Disk Free (complete)</div>
          <div className="mt-0.5 text-zinc-100">
            {summary.queue.diskspace2_norm || "Unknown"}
          </div>
        </div>
      </div>

      {summary.queue.slots.length > 0 && (
        <CollapsibleSection
          title="Queue"
          meta={`${summary.queue.slots.length}`}
          isExpanded={queueExpanded}
          onToggle={toggleQueue}
        >
          <div className="space-y-1.5">
            {summary.queue.slots.slice(0, 10).map((slot) => (
              <div
                key={slot.nzo_id}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="truncate font-medium text-zinc-200">
                  {slot.filename || "Unnamed download"}
                </div>
                <div className="mt-1 flex items-center justify-between text-zinc-400">
                  <span>{slot.status || "Unknown"}</span>
                  <span>{slot.percentage || "0"}%</span>
                </div>
                <div className="mt-1 flex items-center justify-between text-zinc-400">
                  <span>{slot.sizeleft || "0 B"} left</span>
                  <span>{slot.timeleft || "0:00:00"}</span>
                </div>
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
                  key={failure.nzo_id}
                  className="rounded-md border border-amber-800/40 bg-amber-900/20 px-3.5 py-2 text-xs text-amber-300"
                >
                  <div className="truncate font-medium">
                    {failure.name || "Unknown release"}
                  </div>
                  {failure.fail_message && (
                    <div className="mt-1 truncate text-amber-200">
                      {failure.fail_message}
                    </div>
                  )}
                </div>
              ))
            ) : (
              <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs text-zinc-400">
                Failed jobs detected, waiting for history details.
              </div>
            )}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
