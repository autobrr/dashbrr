import { useCallback, useState } from 'react';
import { ServiceType } from '../types/service';
import { useConfiguration } from '../contexts/useConfiguration';
import { toast } from 'react-hot-toast';

interface PendingService {
  type: ServiceType;
  name: string;
  instanceId: string;
  displayName: string;
}

interface ServiceConfig {
  url: string;
  apiKey: string;
  displayName: string;
}

export const useServiceManagement = () => {
  const { configurations, updateConfiguration, deleteConfiguration } = useConfiguration();
  const [showTailscaleConfig, setShowTailscaleConfig] = useState(false);
  const [showServiceConfig, setShowServiceConfig] = useState(false);
  const [pendingService, setPendingService] = useState<PendingService | null>(null);

  const addServiceInstance = useCallback(async (templateType: ServiceType, templateName: string) => {
    if (templateType === 'tailscale') {
      setShowTailscaleConfig(true);
      return;
    }

    const existingInstances = Object.keys(configurations)
      .filter(key => key.startsWith(`${templateType}-`))
      .length;
    const instanceNumber = existingInstances + 1;
    const instanceId = `${templateType}-${instanceNumber}`;
    const displayName = `${templateName}${instanceNumber > 1 ? ` ${instanceNumber}` : ''}`;

    setPendingService({
      type: templateType,
      name: templateName,
      instanceId,
      displayName
    });
    setShowServiceConfig(true);
  }, [configurations]);

  const confirmServiceAddition = useCallback(async (url: string, apiKey: string) => {
    if (!pendingService) return;

    try {
      await updateConfiguration(pendingService.instanceId, {
        url,
        apiKey,
        displayName: pendingService.displayName
      } as ServiceConfig);
      
      toast.success(`Added new ${pendingService.name} instance`);
      setShowServiceConfig(false);
      setPendingService(null);
    } catch (err) {
      toast.error('Failed to add service instance');
      console.error('Error adding service:', err);
    }
  }, [pendingService, updateConfiguration]);

  const cancelServiceAddition = useCallback(() => {
    setShowServiceConfig(false);
    setPendingService(null);
  }, []);

  const removeServiceInstance = useCallback(async (instanceId: string) => {
    try {
      await deleteConfiguration(instanceId);
      toast.success('Service instance removed');
    } catch (err) {
      toast.error('Failed to remove service instance');
      console.error('Error removing service:', err);
    }
  }, [deleteConfiguration]);

  return {
    addServiceInstance,
    removeServiceInstance,
    showTailscaleConfig,
    setShowTailscaleConfig,
    showServiceConfig,
    pendingService,
    confirmServiceAddition,
    cancelServiceAddition
  };
};
