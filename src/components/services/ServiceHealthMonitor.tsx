import { memo } from "react";
import { ServiceGrid } from "./ServiceGrid";
import { useServiceData } from "../../hooks/useServiceData";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import LoadingSkeleton from "../shared/LoadingSkeleton";

export const ServiceHealthMonitor = memo(() => {
  const { removeServiceInstance } = useServiceManagement();
  const { services, isLoading } = useServiceData();

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6 px-0 py-6">
        {[...Array(4)].map((_, i) => (
          <LoadingSkeleton key={i} />
        ))}
      </div>
    );
  }

  // Filter out tailscale services
  const displayServices = services.filter(
    (service) => service.type !== "tailscale"
  );

  return (
    <div className="space-y-6">
      <ServiceGrid
        services={displayServices}
        onRemoveService={removeServiceInstance}
        isConnected={true}
        isLoading={isLoading}
      />
    </div>
  );
});

ServiceHealthMonitor.displayName = "ServiceHealthMonitor";
