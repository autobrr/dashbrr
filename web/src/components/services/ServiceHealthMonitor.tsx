/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { memo } from "react";
import { ServiceGrid } from "./ServiceGrid";
import { useServiceData } from "../../hooks/useServiceData";
import { useServiceManagement } from "../../hooks/useServiceManagement";

export const ServiceHealthMonitor = memo(() => {
  const { removeServiceInstance } = useServiceManagement();
  const { services, isLoading } = useServiceData();

  // The mesh VPN services (Tailscale, NetBird) live in the page header, so the
  // grid leaves them out.
  const displayServices = services.filter(
    (service) => service.type !== "tailscale" && service.type !== "netbird"
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
