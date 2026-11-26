/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { BazarrProviderStatus } from "../../../types/service";

export function isBazarrProviderIssue(provider: BazarrProviderStatus): boolean {
  return (provider.status ?? "").trim().toLowerCase() !== "good";
}

export function getBazarrIssueProviders(
  providers: BazarrProviderStatus[] = []
): BazarrProviderStatus[] {
  return providers.filter(isBazarrProviderIssue);
}

export function getBazarrProvidersWithIssuesCount(
  providers: BazarrProviderStatus[] = [],
  badgeCount = 0
): number {
  return Math.max(getBazarrIssueProviders(providers).length, badgeCount || 0);
}

export function getBazarrSignalRStatusColor(state?: string): string {
  switch ((state ?? "").trim().toUpperCase()) {
    case "LIVE":
      return "text-green-400";
    case "DOWN":
      return "text-red-400";
    default:
      return "text-amber-400";
  }
}
