/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { toast } from "react-hot-toast";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrMessage } from "../common/ArrMessage";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import AnimatedModal from "../../ui/AnimatedModal";
import { runGeneralAction } from "../../../api/general";
import type { GeneralActionDescriptor } from "../../../types/service";
import { buildGeneralStatsView } from "./generalStatsView";

interface GeneralStatsProps {
  instanceId: string;
}

export const GeneralStats: React.FC<GeneralStatsProps> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";
  const [pendingAction, setPendingAction] = React.useState<GeneralActionDescriptor | null>(
    null
  );
  const [runningActionId, setRunningActionId] = React.useState<string | null>(null);

  const runAction = async (action: GeneralActionDescriptor, confirm: boolean) => {
    setRunningActionId(action.id);
    try {
      const result = await runGeneralAction(instanceId, action.id, confirm);
      toast.success(`${action.label} ran (status ${result.status})`);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Action failed";
      toast.error(message);
    } finally {
      setRunningActionId(null);
      setPendingAction(null);
    }
  };

  const handleActionClick = (action: GeneralActionDescriptor) => {
    if (action.confirm) {
      setPendingAction(action);
      return;
    }
    void runAction(action, false);
  };

  if (isLoading) {
    return <StatsSkeleton rows={1} showRight={false} />;
  }

  if (!service) {
    return null;
  }

  const view = buildGeneralStatsView(service);

  return (
    <div className="space-y-4">
      {view.showMessage && (
        <ArrMessage status={service.status} message={view.message} />
      )}

      {view.tiles.length > 0 && (
        <div className="grid grid-cols-2 gap-1.5 @md:grid-cols-4">
          {view.tiles.map((tile) => (
            <div
              key={tile.key}
              className="flex flex-col justify-between rounded-md bg-gray-850/95 px-3.5 py-2 text-xs"
            >
              <span className="text-gray-400">{tile.label}</span>
              <span className="text-sm font-bold text-gray-100">
                {tile.display}
                {tile.unit ? ` ${tile.unit}` : ""}
              </span>
            </div>
          ))}
        </div>
      )}

      {view.actions.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {view.actions.map((action) => (
            <button
              key={action.id}
              type="button"
              onClick={() => handleActionClick(action)}
              disabled={runningActionId === action.id}
              className="rounded-md bg-gray-850/95 px-3 py-1.5 text-xs font-medium text-gray-200 transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {runningActionId === action.id ? "Running..." : action.label}
            </button>
          ))}
        </div>
      )}

      {view.detailFields.length > 0 && (
        <div className="text-xs rounded-md bg-gray-850/95 p-3.5">
          {view.detailFields.map((field) => (
            <div key={field.key} className="flex justify-between gap-4 py-0.5">
              <span className="text-gray-600 dark:text-gray-400">{field.label}</span>
              <span className="text-gray-700 dark:text-gray-200 truncate">{field.value}</span>
            </div>
          ))}
        </div>
      )}

      <AnimatedModal
        isOpen={pendingAction !== null}
        onClose={() => setPendingAction(null)}
        title="Confirm action"
        maxWidth="sm"
      >
        <div className="mt-2">
          <p className="text-zinc-600 dark:text-zinc-300">
            Are you sure you want to run &quot;{pendingAction?.label}&quot;?
          </p>
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            className="inline-flex justify-center rounded-md border border-zinc-300 dark:border-zinc-600 px-4 py-2 text-sm font-medium text-zinc-700 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 transition-colors duration-200"
            onClick={() => setPendingAction(null)}
          >
            Cancel
          </button>
          <button
            type="button"
            className="inline-flex justify-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 transition-colors duration-200"
            onClick={() => pendingAction && void runAction(pendingAction, true)}
          >
            Confirm
          </button>
        </div>
      </AnimatedModal>
    </div>
  );
};
