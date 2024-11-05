import React from "react";
import { Service } from "../../types/service";
import { StatusIcon } from "../ui/StatusIcon";

interface StatusCountersProps {
  services: Service[];
}

export const StatusCounters: React.FC<StatusCountersProps> = ({ services }) => {
  const counts = {
    error: 0,
    warning: 0,
    ok: 0,
    online: 0,
    offline: 0,
    healthy: 0,
    pending: 0,
    loading: 0,
  };

  services.forEach((service) => {
    const status = service.status.toLowerCase();
    if (status in counts) {
      counts[status as keyof typeof counts]++;
    }
  });

  return (
    <div className="flex items-center space-x-4">
      {counts.error > 0 && (
        <div className="flex items-center">
          <StatusIcon status="error" />
          <span className="text-sm text-red-500 font-medium ml-2">
            {counts.error}
          </span>
        </div>
      )}
      {counts.warning > 0 && (
        <div className="flex items-center">
          <StatusIcon status="warning" />
          <span className="text-sm text-yellow-500 font-medium ml-2">
            {counts.warning}
          </span>
        </div>
      )}
      {(counts.online > 0 || counts.healthy > 0) && (
        <div className="flex items-center">
          <StatusIcon status="online" />
          <span className="text-sm text-green-500 font-medium ml-2">
            {counts.online + counts.healthy}
          </span>
        </div>
      )}
    </div>
  );
};
