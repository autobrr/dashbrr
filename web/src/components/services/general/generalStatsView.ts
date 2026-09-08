/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

// Pure view-building logic for GeneralStats, kept separate from the React
// component so it can be unit tested without a DOM (mirrors the
// uptimeKumaView.ts pattern used for UptimeKumaStats).

import type { GeneralActionDescriptor, Service } from "../../../types/service";

const MAX_DETAIL_FIELDS = 12;

export interface GeneralStatTile {
  key: string;
  label: string;
  display: string;
  unit?: string;
}

export interface GeneralDetailField {
  key: string;
  label: string;
  value: string;
}

export interface GeneralStatsView {
  message?: string;
  showMessage: boolean;
  tiles: GeneralStatTile[];
  actions: GeneralActionDescriptor[];
  detailFields: GeneralDetailField[];
}

export const prettifyKey = (key: string): string =>
  key
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());

// The internal "general_stats" event is merged into service.stats.general
// by hooks/serviceData/merge.ts; service.health (only set for the external
// "health" event) doesn't normally carry it, but fall back to it too in
// case a payload ever sets stats.general directly on the health event.
const resolveGeneralStats = (service: Service) =>
  service.stats?.general ?? service.health?.stats?.general;

// The legacy key/value list stays at the top-level details.general - the
// health job (D2) writes it there and buildGeneralServiceUpdate (D3)
// deliberately leaves it untouched.
const resolveDetailFields = (service: Service): Record<string, string | number | boolean> =>
  service.details?.general ?? {};

export const buildGeneralStatsView = (service: Service): GeneralStatsView => {
  const generalStats = resolveGeneralStats(service);

  const showMessage = Boolean(service.message) || service.status !== "online";

  const tiles: GeneralStatTile[] = Object.entries(generalStats?.stats ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, stat]) => ({
      key,
      label: key,
      display: stat.display,
      unit: stat.unit,
    }));

  const actions: GeneralActionDescriptor[] = generalStats?.actions ?? [];

  const detailFields: GeneralDetailField[] = Object.entries(resolveDetailFields(service))
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(0, MAX_DETAIL_FIELDS)
    .map(([key, value]) => ({
      key,
      label: prettifyKey(key),
      value: String(value),
    }));

  return {
    message: service.message,
    showMessage,
    tiles,
    actions,
    detailFields,
  };
};
