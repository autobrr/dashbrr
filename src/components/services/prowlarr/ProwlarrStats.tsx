import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { StatsLoadingSkeleton } from "../../shared/StatsLoadingSkeleton";

interface ProwlarrStatsProps {
  instanceId: string;
}

export const ProwlarrStats: React.FC<ProwlarrStatsProps> = ({ instanceId }) => {
  const { services, isLoading } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);

  if (isLoading) {
    return (
      <div className="mt-2 grid grid-cols-2 gap-4">
        <StatsLoadingSkeleton />
        <StatsLoadingSkeleton />
      </div>
    );
  }

  if (!service?.stats?.prowlarr?.stats || !service?.stats?.prowlarr?.indexers) {
    return null;
  }

  const { indexers } = service.stats.prowlarr;
  const activeIndexers = indexers.filter((i) => i.enable).length;

  return (
    <>
      <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
        Indexer Stats:
      </div>
      <div className="flex flex-row mt-2 gap-16 items-center justify-start text-xs rounded-md text-gray-700 dark:text-gray-300 bg-gray-850/95 p-4">
        <div>
          <div className="text-md font-semibold">{activeIndexers}</div>
          <div className="text-xs text-gray-500">Active Indexers</div>
        </div>
        <div>
          <div className="text-md font-semibold">{indexers.length}</div>
          <div className="text-xs text-gray-500">Total Indexers</div>
        </div>
      </div>
    </>
  );
};

export default ProwlarrStats;
