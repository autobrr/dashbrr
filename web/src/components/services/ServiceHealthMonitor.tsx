/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { memo } from "react";
import { ServiceGrid } from "./ServiceGrid";
import { useServiceData } from "../../hooks/useServiceData";
import { useServiceManagement } from "../../hooks/useServiceManagement";
import { isMeshNetworkType } from "../../utils/meshNetwork";

export const ServiceHealthMonitor = memo(() => {
  const { removeServiceInstance } = useServiceManagement();
  const { services, isLoading } = useServiceData();

  const displayServices = services.filter((service) => !isMeshNetworkType(service.type));

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
