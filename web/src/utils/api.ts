/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { readErrorMessage } from "./http";

// Service-specific timeouts
const SERVICE_TIMEOUTS: Record<string, number> = {
  '/api/autobrr/stats': 60000,      // 1 minute for autobrr stats
  '/api/autobrr/irc': 300000,       // 5 minutes for autobrr IRC
  '/api/autobrr/releases': 60000,   // 1 minute for autobrr releases
  '/api/plex/sessions': 5000,       // 5 seconds for plex sessions
  '/api/jellyfin': 30000,           // 30 seconds for jellyfin summary
  '/api/uptimekuma': 30000,         // 30 seconds for uptime kuma summary
  '/api/maintainerr': 600000,       // 10 minutes for maintainerr
  '/api/overseerr': 30000,          // 30 seconds for overseerr
  '/api/radarr': 60000,             // 1 minute for radarr
  '/api/sonarr': 60000,             // 1 minute for sonarr
  '/api/lidarr': 60000,             // 1 minute for lidarr
  '/api/readarr': 60000,            // 1 minute for readarr
  '/api/bazarr': 60000,             // 1 minute for bazarr
  '/api/sabnzbd': 60000,            // 1 minute for sabnzbd
  '/api/nzbget': 60000,             // 1 minute for nzbget
  '/api/prowlarr': 60000,           // 1 minute for prowlarr
  '/api/health': 600000,            // 10 minutes for health checks
};

const getDefaultHeaders = (): Record<string, string> => ({
  "Content-Type": "application/json",
});

const createRequest = (method: string, data?: unknown): RequestInit => {
  const options: RequestInit = {
    method,
    headers: getDefaultHeaders(),
    credentials: 'include',
  };

  if (data) {
    options.body = JSON.stringify(data);
  }

  return options;
};

const getTimeoutForPath = (path: string): number => {
  // Check for exact matches first
  if (SERVICE_TIMEOUTS[path]) {
    return SERVICE_TIMEOUTS[path];
  }

  // Check for partial matches
  for (const [key, timeout] of Object.entries(SERVICE_TIMEOUTS)) {
    if (path.includes(key)) {
      return timeout;
    }
  }

  // Default timeout of 8 seconds
  return 8000;
};

// Utility to unregister service worker
const unregisterServiceWorker = async (): Promise<void> => {
  if ('serviceWorker' in navigator) {
    const registrations = await navigator.serviceWorker.getRegistrations();
    for (const registration of registrations) {
      await registration.unregister();
    }
  }
};

// Check if the path is an authentication-related endpoint
const isNoRedirectOn401Endpoint = (path: string): boolean => {
  // Endpoints where a 401 should be surfaced to the caller (bad creds, etc),
  // not treated as "session expired, redirect to /login".
  const paths = [
    '/api/auth/login',
    '/api/auth/register',
    '/api/auth/registration-status',
    '/api/auth/config',
    '/api/auth/oidc/login',
    '/api/auth/oidc/logout',
  ];
  return paths.some((p) => path.includes(p));
};

// Track auth state changes to prevent cascading 401 handlers
let isHandlingAuth = false;

const handleRequest = async <T>(
  path: string,
  options: RequestInit,
  retryCount = 0,
  customTimeout?: number
): Promise<T> => {
  const timeout = customTimeout || getTimeoutForPath(path);
  
  try {
    const apiPath = path.startsWith('/api') ? path : `/api${path}`;
    const url = apiPath;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    const requestOptions = {
      ...options,
      signal: controller.signal,
    };

    const response = await fetch(url, requestOptions);
    clearTimeout(timeoutId);

    if (response.status === 401) {
      // Prevent multiple auth handlers from running simultaneously
      if (isHandlingAuth) {
        throw new Error('Authentication in progress');
      }

      if (isNoRedirectOn401Endpoint(apiPath)) {
        throw new Error((await readErrorMessage(response)) || "Unauthorized");
      }

      isHandlingAuth = true;
      localStorage.removeItem('auth_type');

      // Dev-only: stale Workbox caches can cause "unstyled Tailwind" reload loops.
      if (import.meta.env.DEV) {
        await unregisterServiceWorker();
      }

      window.location.href = '/login';
      throw new Error('Authentication required');
    }

    if (!response.ok) {
      if (response.status === 429 && retryCount < 3) {
        const retryAfter = parseInt(response.headers.get('Retry-After') || '0');
        const waitTime = retryAfter ? retryAfter * 1000 : Math.min(1000 * Math.pow(2, retryCount), 30000);
        await new Promise(resolve => setTimeout(resolve, waitTime));
        return handleRequest<T>(path, options, retryCount + 1, customTimeout);
      }
      throw new Error(
        (await readErrorMessage(response)) ||
          `HTTP error! status: ${response.status}`
      );
    }

    if (response.status === 204) {
      return {} as T;
    }

    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      return (await response.json()) as T;
    }

    const text = await response.text();
    if (!text) return {} as T;
    return text as unknown as T;
  } catch (error) {
    if (error instanceof Error) {
      if (error.name === 'AbortError') {
        throw new Error(`Request timed out after ${timeout}ms`);
      }
      throw error;
    }
    throw error;
  }
};

export const api = {
  get: async <T>(path: string, timeout?: number): Promise<T> => {
    return handleRequest<T>(path, createRequest('GET'), 0, timeout);
  },

  post: async <T>(path: string, data?: unknown, timeout?: number): Promise<T> => {
    return handleRequest<T>(path, createRequest('POST', data), 0, timeout);
  },

  put: async <T>(path: string, data: unknown, timeout?: number): Promise<T> => {
    return handleRequest<T>(path, createRequest('PUT', data), 0, timeout);
  },

  delete: async <T>(path: string, timeout?: number): Promise<T> => {
    return handleRequest<T>(path, createRequest('DELETE'), 0, timeout);
  },
};
