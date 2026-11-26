/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useState } from "react";
import { addServiceSetup, ServiceSetup } from "../config/serviceSetup";
import { ServiceConfigInput, ServiceType } from "../types/service";
import { useConfiguration } from "../contexts/useConfiguration";
import { toast } from "react-hot-toast";

interface PendingService {
  name: string;
  setup: ServiceSetup;
}

export const useServiceManagement = () => {
  const { configurations, updateConfiguration, deleteConfiguration } = useConfiguration();
  const [showServiceConfig, setShowServiceConfig] = useState(false);
  const [pendingService, setPendingService] = useState<PendingService | null>(null);

  const addServiceInstance = useCallback((type: ServiceType, name: string) => {
    setPendingService({ name, setup: addServiceSetup(type, name, configurations) });
    setShowServiceConfig(true);
  }, [configurations]);

  const confirmServiceAddition = useCallback(async (input: ServiceConfigInput) => {
    if (!pendingService) return;

    await updateConfiguration(pendingService.setup.instanceId, input);
    toast.success("Added new service instance");
    setShowServiceConfig(false);
  }, [pendingService, updateConfiguration]);

  const cancelServiceAddition = useCallback(() => {
    setShowServiceConfig(false);
  }, []);

  const removeServiceInstance = useCallback(async (instanceId: string) => {
    try {
      await deleteConfiguration(instanceId);
      toast.success("Service instance removed");
    } catch (err) {
      toast.error("Failed to remove service instance");
      console.error("Error removing service:", err);
    }
  }, [deleteConfiguration]);

  return {
    addServiceInstance,
    removeServiceInstance,
    showServiceConfig,
    pendingService,
    confirmServiceAddition,
    cancelServiceAddition,
  };
};
