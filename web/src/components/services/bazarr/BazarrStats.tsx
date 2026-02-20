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

interface BazarrStatsProps {
  instanceId: string;
}

export const BazarrStats: React.FC<BazarrStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";
  const { isExpanded: providersExpanded, toggle: toggleProviders } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "bazarr:providers"),
      true
    );
  const { isExpanded: healthExpanded, toggle: toggleHealth } =
    useCollapsiblePreference(
      serviceSectionCollapseKey(instanceId, "bazarr:health_issues"),
      true
    );

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  const summary = service.stats?.bazarr?.summary;
  const badges = summary?.badges ?? {
    episodes: 0,
    movies: 0,
    providers: 0,
    status: 0,
    sonarr_signalr: "",
    radarr_signalr: "",
    announcements: 0,
  };
  const providers = summary?.providers ?? [];
  const healthIssues = summary?.healthIssues ?? [];

  const providersWithIssues = Math.max(providers.length, badges.providers || 0);
  const systemHealthIssues = Math.max(healthIssues.length, badges.status || 0);
  const message = combineServiceMessage(service);

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      <div>
        <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
          Subtitles Backlog:
        </div>
        <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          <div className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2">
            <span className="text-xs font-normal text-gray-200">
              Missing Episodes
            </span>
            <span className="text-sm font-bold text-gray-200">
              {badges.episodes || 0}
            </span>
          </div>
          <div className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2">
            <span className="text-xs font-normal text-gray-200">
              Missing Movies
            </span>
            <span className="text-sm font-bold text-gray-200">
              {badges.movies || 0}
            </span>
          </div>
          <div className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2">
            <span className="text-xs font-normal text-gray-200">
              Providers with Issues
            </span>
            <span className="text-sm font-bold text-amber-400">
              {providersWithIssues}
            </span>
          </div>
          <div className="flex items-center justify-between text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 px-3.5 py-2">
            <span className="text-xs font-normal text-gray-200">
              System Health Issues
            </span>
            <span className="text-sm font-bold text-amber-400">
              {systemHealthIssues}
            </span>
          </div>
        </div>
      </div>

      {(badges.sonarr_signalr || badges.radarr_signalr) && (
        <div className="flex flex-wrap gap-2 text-xs text-gray-300">
          {badges.sonarr_signalr && (
            <span className="rounded-md bg-gray-850/95 px-2 py-1">
              Sonarr SignalR:{" "}
              <span className="font-semibold text-green-400">
                {badges.sonarr_signalr}
              </span>
            </span>
          )}
          {badges.radarr_signalr && (
            <span className="rounded-md bg-gray-850/95 px-2 py-1">
              Radarr SignalR:{" "}
              <span className="font-semibold text-green-400">
                {badges.radarr_signalr}
              </span>
            </span>
          )}
        </div>
      )}

      {providers.length > 0 && (
        <CollapsibleSection
          title="Providers with issues:"
          meta={`${providers.length}`}
          isExpanded={providersExpanded}
          onToggle={toggleProviders}
        >
          <div className="space-y-2">
            {providers.slice(0, 10).map((provider) => (
              <div
                key={`${provider.name}:${provider.status}:${provider.retry}`}
                className="flex items-center justify-between rounded-md bg-gray-850/95 px-3.5 py-2 text-xs"
              >
                <span className="font-medium text-gray-200">{provider.name}</span>
                <span className="text-gray-400">
                  {provider.status}
                  {provider.retry && provider.retry !== "-" ? ` • Retry ${provider.retry}` : ""}
                </span>
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}

      {healthIssues.length > 0 && (
        <CollapsibleSection
          title="Health issues:"
          meta={`${healthIssues.length}`}
          isExpanded={healthExpanded}
          onToggle={toggleHealth}
        >
          <div className="space-y-2">
            {healthIssues.slice(0, 10).map((issue, index) => (
              <div
                key={`${issue.object}:${issue.issue}:${index}`}
                className="rounded-md border border-amber-800/40 bg-amber-900/20 px-3.5 py-2 text-xs text-amber-300"
              >
                <div className="font-medium">{issue.object || "System"}</div>
                <div>{issue.issue || "Unknown issue"}</div>
              </div>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
};
