/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

interface RequestOptions {
  method: string;
  headers?: Record<string, string>;
  credentials?: RequestCredentials;
  body?: string;
}

// Service-specific timeouts
const SERVICE_TIMEOUTS: Record<string, number> = {
  '/api/autobrr/stats': 20000,    // 10 seconds for autobrr stats
  '/api/autobrr/irc': 20000,       // 5 seconds for autobrr IRC
  '/api/plex/sessions': 20000,    // 10 seconds for plex sessions
  '/api/maintainerr': 20000,      // 10 seconds for maintainerr
  '/api/overseerr': 20000,        // 10 seconds for overseerr
  '/api/radarr': 20000,           // 10 seconds for radarr
  '/api/health': 20000,           // 10 seconds for health checks
};

// Request queue for handling requests during auth initialization
class RequestQueue {
  private static instance: RequestQueue;
  private queue: Array<() => Promise<unknown>> = [];
  private isProcessing = false;
  private concurrentRequests = 0;
  private readonly MAX_CONCURRENT = 4;  // Allow up to 4 concurrent requests

  private constructor() {}

  static getInstance(): RequestQueue {
    if (!RequestQueue.instance) {
      RequestQueue.instance = new RequestQueue();
    }
    return RequestQueue.instance;
  }

  async add<T>(request: () => Promise<T>): Promise<T> {
    if (this.concurrentRequests < this.MAX_CONCURRENT) {
      this.concurrentRequests++;
      try {
        return await request();
      } finally {
        this.concurrentRequests--;
        this.processQueue();
      }
    }

    return new Promise((resolve, reject) => {
      this.queue.push(async () => {
        try {
          const result = await request();
          resolve(result as T);
        } catch (error) {
          reject(error);
        }
      });
      this.processQueue();
    });
  }

  private async processQueue() {
    if (this.isProcessing || this.queue.length === 0) return;

    this.isProcessing = true;
    while (this.queue.length > 0 && this.concurrentRequests < this.MAX_CONCURRENT) {
      const request = this.queue.shift();
      if (request) {
        this.concurrentRequests++;
        try {
          await request();
        } catch (error) {
          console.error('[RequestQueue] Error processing queued request:', error);
        } finally {
          this.concurrentRequests--;
        }
      }
    }
    this.isProcessing = false;
  }
}

const getAuthHeaders = (): Record<string, string> => {
  const token = localStorage.getItem('access_token');
  return {
    'Authorization': token ? `Bearer ${token}` : '',
    'Content-Type': 'application/json',
  };
};

const createRequest = (method: string, data?: unknown): RequestOptions => {
  const options: RequestOptions = {
    method,
    headers: getAuthHeaders(),
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
const isAuthEndpoint = (path: string): boolean => {
  const authPaths = [
    '/api/auth',
    '/api/login',
    '/api/verify',
    '/api/userinfo',
    '/api/token'
  ];
  return authPaths.some(authPath => path.includes(authPath));
};

const handleRequest = async <T>(
  path: string,
  options: RequestOptions,
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
      // Only trigger logout for authentication-related 401s
      if (isAuthEndpoint(path)) {
        localStorage.removeItem('access_token');
        localStorage.removeItem('id_token');
        localStorage.removeItem('auth_type');
        await unregisterServiceWorker();
        window.location.href = '/login';
        throw new Error('Authentication required');
      } else {
        // For service-related 401s (like invalid API keys), just throw an error
        const errorData = await response.json().catch(() => ({ error: 'Unauthorized' }));
        throw new Error(errorData.error || 'Service authentication failed');
      }
    }

    if (!response.ok) {
      if (response.status === 429 && retryCount < 3) {
        const retryAfter = parseInt(response.headers.get('Retry-After') || '0');
        const waitTime = retryAfter ? retryAfter * 1000 : Math.min(1000 * Math.pow(2, retryCount), 30000);
        await new Promise(resolve => setTimeout(resolve, waitTime));
        return handleRequest<T>(path, options, retryCount + 1, customTimeout);
      }
      const errorData = await response.json().catch(() => ({ error: `HTTP error! status: ${response.status}` }));
      throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
    }

    const text = await response.text();
    if (!text) {
      return {} as T;
    }
    
    return JSON.parse(text);
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
    const requestQueue = RequestQueue.getInstance();
    return requestQueue.add(() => handleRequest<T>(path, createRequest('GET'), 0, timeout));
  },

  post: async <T>(path: string, data?: unknown, timeout?: number): Promise<T> => {
    const requestQueue = RequestQueue.getInstance();
    return requestQueue.add(() => handleRequest<T>(path, createRequest('POST', data), 0, timeout));
  },

  put: async <T>(path: string, data: unknown, timeout?: number): Promise<T> => {
    const requestQueue = RequestQueue.getInstance();
    return requestQueue.add(() => handleRequest<T>(path, createRequest('PUT', data), 0, timeout));
  },

  delete: async <T>(path: string, timeout?: number): Promise<T> => {
    const requestQueue = RequestQueue.getInstance();
    return requestQueue.add(() => handleRequest<T>(path, createRequest('DELETE'), 0, timeout));
  },
};

export const getEventSourceUrl = (path: string): string => {
  const apiPath = path.startsWith('/api') ? path : `/api${path}`;
  return `${window.location.origin}${apiPath}`;
};
