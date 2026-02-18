/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrMessage } from "../common/ArrMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { QuiInstanceTransfer } from "../../../types/service";

interface QuiStatsProps {
  instanceId: string;
}

const formatBytes = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let current = value;
  let unit = 0;
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024;
    unit++;
  }
  return `${current.toFixed(current >= 100 ? 0 : 1)} ${units[unit]}`;
};

const formatSpeed = (value: number) => `${formatBytes(value)}/s`;

const toSpeedScore = (transfer: QuiInstanceTransfer) =>
  transfer.downloadSpeed + transfer.uploadSpeed;

export const QuiStats: React.FC<QuiStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);

  const quiStats = service?.stats?.qui;
  const summary = service?.details?.qui?.summary;

  const transfers = [...(quiStats?.transfers ?? [])]
    .sort((a, b) => toSpeedScore(b) - toSpeedScore(a))
    .slice(0, 5);

  const isInitialLoading =
    service?.status === "loading" &&
    !summary &&
    (quiStats?.instances?.length ?? 0) === 0;

  if (isInitialLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const message = combineServiceMessage(service);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      {summary && (
        <div className="grid grid-cols-2 gap-1.5">
          <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
            <div className="text-zinc-400">Instances</div>
            <div className="mt-0.5 text-zinc-100">
              {summary.connectedInstances}/{summary.activeInstances} active
            </div>
          </div>
          <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
            <div className="text-zinc-400">Combined Speed</div>
            <div className="mt-0.5 text-zinc-100">
              {formatSpeed(summary.downloadSpeed + summary.uploadSpeed)}
            </div>
            <div className="text-zinc-400">
              {formatSpeed(summary.downloadSpeed)} down · {formatSpeed(summary.uploadSpeed)} up
            </div>
          </div>
          <div className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs">
            <div className="text-zinc-400">Combined Data</div>
            <div className="mt-0.5 text-zinc-100">
              {formatBytes(summary.downloaded + summary.uploaded)}
            </div>
            <div className="text-zinc-400">
              {formatBytes(summary.downloaded)} down · {formatBytes(summary.uploaded)} up
            </div>
          </div>
        </div>
      )}

      {transfers.length > 0 && (
        <div>
          <div className="mb-2 text-xs font-semibold text-zinc-300">
            Active qBittorrent Instances
          </div>
          <div className="space-y-1.5">
            {transfers.map((transfer) => (
              <div
                key={transfer.instanceId}
                className="rounded-md bg-zinc-900/80 px-3.5 py-2 text-xs"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium text-zinc-200">
                    {transfer.name}
                  </span>
                  <span
                    className={`rounded px-1.5 py-0.5 text-[10px] uppercase tracking-wide ${
                      transfer.connected
                        ? "bg-emerald-500/15 text-emerald-300"
                        : "bg-amber-500/15 text-amber-200"
                    }`}
                  >
                    {transfer.connected ? "Connected" : "Disconnected"}
                  </span>
                </div>
                <div className="mt-1 flex items-center justify-between text-zinc-400">
                  <span>Down {formatSpeed(transfer.downloadSpeed)}</span>
                  <span>Up {formatSpeed(transfer.uploadSpeed)}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
