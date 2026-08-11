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
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import { useTranslation } from "react-i18next";

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
  const { t } = useTranslation();
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "qui:active_instances"),
    true
  );

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
        <div className="grid grid-cols-2 gap-4 bg-zinc-900/50 rounded-lg p-3 @md:p-4 mb-4">
          <div>
            <p className="text-xs font-medium text-zinc-400 mb-1">
              {t("qui.combined_speed", "Combined Speed")}
            </p>
            <p className="text-sm font-semibold text-zinc-100">
              {formatSpeed(summary.downloadSpeed + summary.uploadSpeed)}
            </p>
            <p className="text-xs text-zinc-500 mt-0.5">
              {formatSpeed(summary.downloadSpeed)} {t("service.down", "down")} · {formatSpeed(summary.uploadSpeed)} {t("service.up", "up")}
            </p>
          </div>
          <div>
            <p className="text-xs font-medium text-zinc-400 mb-1">
              {t("qui.combined_data", "Combined Data (all-time)")}
            </p>
            <p className="text-sm font-semibold text-zinc-100">
              {formatBytes(summary.downloaded + summary.uploaded)}
            </p>
            <p className="text-xs text-zinc-500 mt-0.5">
              {formatBytes(summary.downloaded)} {t("service.down", "down")} · {formatBytes(summary.uploaded)} {t("service.up", "up")}
            </p>
          </div>
        </div>
      )}

      {transfers.length > 0 && (
        <CollapsibleSection
          title={t("qui.active_instances", "Active qBittorrent Instances")}
          isExpanded={isExpanded}
          onToggle={toggle}
        >
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
                  <div className="flex items-center gap-2">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                        transfer.connected
                          ? "bg-emerald-500/10 text-emerald-500"
                          : "bg-red-500/10 text-red-500"
                      }`}
                    >
                      {transfer.connected ? t("service.connected", "CONNECTED") : t("service.disconnected", "DISCONNECTED")}
                    </span>
                  </div>
                </div>
                <div className="flex justify-between text-xs text-zinc-500 mt-1">
                  <span>{t("service.down", "Down")} {formatSpeed(transfer.downloadSpeed)}</span>
                  <span>{t("service.up", "Up")} {formatSpeed(transfer.uploadSpeed)}</span>
                </div>
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
