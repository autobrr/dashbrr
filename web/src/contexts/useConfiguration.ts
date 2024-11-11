/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useContext } from 'react';
import { ConfigurationContext } from './context';

export const useConfiguration = () => {
  const context = useContext(ConfigurationContext);
  if (!context) {
    throw new Error('useConfiguration must be used within a ConfigurationProvider');
  }

  const validateServiceConfig = async (type: string, url: string, apiKey?: string) => {
    try {
      // Ensure URL is properly formatted
      const formattedUrl = url.endsWith('/') ? url.slice(0, -1) : url;
      
      // Build query parameters
      const params = new URLSearchParams();
      params.append('url', formattedUrl);
      if (apiKey) {
        params.append('apiKey', apiKey);
      }
      
      // Construct the health check URL
      const healthCheckUrl = `/health/${type.toLowerCase()}?${params.toString()}`;
      
      const response = await fetch(healthCheckUrl, {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error(`Service validation failed with status: ${response.status}`);
      }

      const contentType = response.headers.get('content-type');
      if (!contentType || !contentType.includes('application/json')) {
        throw new Error('Invalid response format from service');
      }

      const data = await response.json();
      return data;
    } catch (error) {
      console.error('Service validation error:', error);
      throw error;
    }
  };

  return {
    ...context,
    validateServiceConfig,
  };
};

export default useConfiguration;
