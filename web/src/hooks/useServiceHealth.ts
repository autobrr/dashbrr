/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useContext } from 'react';
import { useServiceData } from './useServiceData';
import { ServiceStatus } from '../types/service';
import { api } from '../utils/api';
import { NotificationContext } from '../contexts/NotificationContext';

type StatusCount = Record<ServiceStatus, number>;

const initialStatusCount: StatusCount = {
  online: 0,
  offline: 0,
  warning: 0,
  error: 0,
  loading: 0,
  pending: 0,
  unknown: 0,
};

interface HealthResponse {
  status: ServiceStatus;
  message?: string;
  version?: string;
  updateAvailable?: boolean;
}

export const useServiceHealth = () => {
  const { services, isLoading, refreshService } = useServiceData();
  const notificationContext = useContext(NotificationContext);

  if (!notificationContext) {
    throw new Error('useServiceHealth must be used within a NotificationProvider');
  }

  const { notifyServiceUpdate, notifyVersionUpdate } = notificationContext;

  const getStatusCounts = useCallback((): StatusCount => {
    return (services || []).reduce(
      (acc, service) => {
        const status = service.status || 'unknown';
        return {
          ...acc,
          [status]: acc[status] + 1,
        };
      },
      { ...initialStatusCount }
    );
  }, [services]);

  const refreshServiceHealth = useCallback(async (instanceId: string) => {
    try {
      const response = await api.get<HealthResponse>(`/api/health/${instanceId}`);
      if (response && response.status) {
        refreshService(instanceId, 'health');
        
        // Find the service to get its name and current version
        const service = services?.find(s => s.instanceId === instanceId);
        if (service) {
          // Handle status change notifications
          if (['error', 'offline', 'warning'].includes(response.status)) {
            notifyServiceUpdate(
              service.name,
              response.status,
              response.message || `Service is ${response.status}`
            );
          }
          // Notify when service comes back online from a problematic state
          else if (
            response.status === 'online' && 
            service.status && 
            ['error', 'offline', 'warning'].includes(service.status)
          ) {
            notifyServiceUpdate(
              service.name,
              'online',
              'Service is now online'
            );
          }

          // Handle version update notifications
          if (
            response.updateAvailable &&
            response.version &&
            service.version &&
            response.version !== service.version
          ) {
            notifyVersionUpdate(
              service.name,
              service.version,
              response.version
            );
          }
        }
      }
      return response;
    } catch (error) {
      console.error(`Error refreshing health for service ${instanceId}:`, error);
      return null;
    }
  }, [refreshService, services, notifyServiceUpdate, notifyVersionUpdate]);

  return {
    services: services || [],
    isLoading,
    refreshServiceHealth,
    statusCounts: getStatusCounts(),
  };
};
