import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";

interface OverseerrStatsProps {
  instanceId: string;
}

export const OverseerrStats: React.FC<OverseerrStatsProps> = ({
  instanceId,
}) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const pendingRequests = service?.stats?.overseerr?.pendingRequests;
  const isLoading = !service || service.status === "loading";
  const error = service?.status === "error" ? service.message : null;

  if (isLoading) {
    return <p className="text-xs text-gray-500">Loading requests...</p>;
  }

  if (error) {
    return <p className="text-xs text-gray-500">Error: {error}</p>;
  }

  return (
    <>
      <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
        Pending Requests:
      </div>
      <div className="mt-2">
        <div className="text-xs rounded-md text-gray-700 dark:text-gray-400 bg-gray-850/95 p-4 space-y-1">
          <div className="">
            {pendingRequests || 0}{" "}
            {pendingRequests === 1 ? "request" : "requests"} awaiting approval
          </div>
        </div>
      </div>
    </>
  );
};
