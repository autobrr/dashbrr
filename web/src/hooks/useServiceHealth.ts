/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useMemo } from 'react';
import { useServiceData } from './useServiceData';
import { ServiceStatus } from '../types/service';
import { api } from '../utils/api';

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

  // Memoize status counts to prevent unnecessary recalculations
  const statusCounts = useMemo((): StatusCount => {
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
      }
      return response;
    } catch (error) {
      console.error(`Error refreshing health for service ${instanceId}:`, error);
      return null;
    }
  }, [refreshService]);

  return {
    services: services || [],
    isLoading,
    refreshServiceHealth,
    statusCounts,
  };
};
