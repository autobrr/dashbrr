/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';

// Error types for better error handling
export enum EventSourceErrorType {
  AUTH = 'auth_error',
  NETWORK = 'network_error',
  RATE_LIMIT = 'rate_limit',
  SERVER = 'server_error',
  UNKNOWN = 'unknown_error',
}

interface EventSourceError extends Error {
  type: EventSourceErrorType;
  retryAfter?: number;
}

interface EventSourceOptions<T> {
  onMessage?: (data: T) => void;
  onError?: (error: EventSourceError) => void;
  reconnectInterval?: number;
  maxRetries?: number;
}

interface EventData {
  type: string;
  [key: string]: unknown;
}

// Extended EventSource type with additional properties
interface ExtendedEventSource extends EventSource {
  status?: number;
  getResponseHeader?: (name: string) => string | null;
}

// Constants for reconnection strategy
const MAX_BACKOFF_DELAY = 30000; // 30 seconds
const BASE_RETRY_DELAY = 5000;   // 5 seconds
const HEALTH_CHECK_INTERVAL = 60000;     // 1 minute
const MESSAGE_TIMEOUT = 300000;          // 5 minutes
const DEFAULT_MAX_RETRIES = 5;
const UPDATE_DEBOUNCE_TIME = 100;       // 100ms debounce for updates

export const useEventSource = <T extends EventData>(
  path: string, 
  options: EventSourceOptions<T> = {}
) => {
  const [isConnected, setIsConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const { isAuthenticated } = useAuth();
  const reconnectTimeoutRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);
  const lastMessageTimeRef = useRef<number>(Date.now());
  const connectionLockRef = useRef<boolean>(false);
  const authErrorRef = useRef<boolean>(false);
  const updateTimeoutRef = useRef<number | null>(null);
  const pendingUpdatesRef = useRef<T[]>([]);

  const {
    onMessage,
    onError,
    reconnectInterval = BASE_RETRY_DELAY,
    maxRetries = DEFAULT_MAX_RETRIES
  } = options;

  // Create custom error
  const createError = (type: EventSourceErrorType, message: string, retryAfter?: number): EventSourceError => {
    const error = new Error(message) as EventSourceError;
    error.type = type;
    if (retryAfter) {
      error.retryAfter = retryAfter;
    }
    return error;
  };

  // Cleanup function with connection lock handling
  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (updateTimeoutRef.current) {
      window.clearTimeout(updateTimeoutRef.current);
      updateTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setIsConnected(false);
    connectionLockRef.current = false;
    pendingUpdatesRef.current = [];
  }, []);

  // Process pending updates with debouncing
  const processPendingUpdates = useCallback(() => {
    if (updateTimeoutRef.current) {
      window.clearTimeout(updateTimeoutRef.current);
    }

    updateTimeoutRef.current = window.setTimeout(() => {
      if (!mountedRef.current) return;

      const updates = pendingUpdatesRef.current;
      pendingUpdatesRef.current = [];

      // Process all accumulated updates at once
      if (updates.length > 0) {
        const lastUpdate = updates[updates.length - 1];
        onMessage?.(lastUpdate);
      }
    }, UPDATE_DEBOUNCE_TIME);
  }, [onMessage]);

  // Calculate backoff delay with jitter
  const getBackoffDelay = useCallback((retryCount: number): number => {
    const baseDelay = Math.min(
      BASE_RETRY_DELAY * Math.pow(2, retryCount),
      MAX_BACKOFF_DELAY
    );
    // Add random jitter (±20% of base delay)
    const jitter = baseDelay * 0.2 * (Math.random() * 2 - 1);
    return Math.max(BASE_RETRY_DELAY, baseDelay + jitter);
  }, []);

  // Connection health check
  useEffect(() => {
    const healthCheckInterval = setInterval(() => {
      if (!mountedRef.current) return;

      const now = Date.now();
      const messageAge = now - lastMessageTimeRef.current;

      // If no message received in MESSAGE_TIMEOUT, reconnect
      if (isConnected && messageAge > MESSAGE_TIMEOUT && eventSourceRef.current) {
        console.warn('[EventSource] No messages received for 5 minutes, reconnecting...');
        cleanup();
        retryCountRef.current = 0; // Reset retry count for fresh connection
        authErrorRef.current = false; // Reset auth error flag
      }
    }, HEALTH_CHECK_INTERVAL);

    return () => {
      clearInterval(healthCheckInterval);
    };
  }, [isConnected, cleanup]);

  // Main EventSource connection logic
  useEffect(() => {
    if (!path || !isAuthenticated || connectionLockRef.current || authErrorRef.current) {
      return;
    }

    const connect = () => {
      if (!mountedRef.current || connectionLockRef.current) return;

      connectionLockRef.current = true;

      try {
        // Get access token from localStorage
        const accessToken = localStorage.getItem('access_token');
        if (!accessToken) {
          const error = createError(
            EventSourceErrorType.AUTH,
            'No access token available for connection'
          );
          console.error('[EventSource]', error.message);
          setIsConnected(false);
          connectionLockRef.current = false;
          authErrorRef.current = true;
          onError?.(error);
          return;
        }

        cleanup(); // Ensure clean slate before connecting

        const url = new URL(path, window.location.origin);
        url.searchParams.append('token', accessToken);
        url.searchParams.append('nocache', Date.now().toString());

        const eventSource = new EventSource(url.toString());
        eventSourceRef.current = eventSource;

        eventSource.onopen = () => {
          if (!mountedRef.current) return;
          
          console.log('[EventSource] Connected successfully');
          setIsConnected(true);
          retryCountRef.current = 0; // Reset retry count on successful connection
          lastMessageTimeRef.current = Date.now();
          authErrorRef.current = false;
        };

        eventSource.onmessage = (event) => {
          if (!mountedRef.current) return;

          try {
            const data = JSON.parse(event.data) as T;
            lastMessageTimeRef.current = Date.now();
            pendingUpdatesRef.current.push(data);
            processPendingUpdates();
          } catch (error) {
            console.error('[EventSource] Error parsing message:', error);
            const parseError = createError(
              EventSourceErrorType.UNKNOWN,
              'Error parsing message'
            );
            onError?.(parseError);
          }
        };

        // Handle health events
        eventSource.addEventListener('health', (event: MessageEvent) => {
          if (!mountedRef.current) return;

          try {
            const data = JSON.parse(event.data) as T;
            lastMessageTimeRef.current = Date.now();
            pendingUpdatesRef.current.push(data);
            processPendingUpdates();
          } catch (error) {
            console.error('[EventSource] Error parsing health event:', error);
            const parseError = createError(
              EventSourceErrorType.UNKNOWN,
              'Error parsing health event'
            );
            onError?.(parseError);
          }
        });

        // Handle keepalive events
        eventSource.addEventListener('keepalive', () => {
          if (!mountedRef.current) return;
          lastMessageTimeRef.current = Date.now();
        });

        eventSource.onerror = (error) => {
          if (!mountedRef.current) return;

          // Determine error type and retry after value
          let errorType = EventSourceErrorType.UNKNOWN;
          let retryAfter: number | undefined;

          if (error instanceof Event && error.target) {
            const target = error.target as ExtendedEventSource;
            const status = target.status;
            
            if (status === 401 || status === 403 || !isAuthenticated || !localStorage.getItem('access_token')) {
              errorType = EventSourceErrorType.AUTH;
            } else if (status === 429) {
              errorType = EventSourceErrorType.RATE_LIMIT;
              const retryHeader = target.getResponseHeader?.('Retry-After');
              retryAfter = retryHeader ? parseInt(retryHeader, 10) : 60;
            } else if (status && status >= 500) {
              errorType = EventSourceErrorType.SERVER;
            } else if (!navigator.onLine || status === 0) {
              errorType = EventSourceErrorType.NETWORK;
            }
          }

          const customError = createError(
            errorType,
            `Connection error: ${errorType}`,
            retryAfter
          );

          console.error('[EventSource] Connection error:', customError);
          setIsConnected(false);
          onError?.(customError);
          cleanup();

          // Handle different error types
          switch (errorType) {
            case EventSourceErrorType.AUTH: {
              console.error('[EventSource] Authentication error detected');
              authErrorRef.current = true;
              return;
            }

            case EventSourceErrorType.RATE_LIMIT: {
              if (retryAfter) {
                reconnectTimeoutRef.current = window.setTimeout(() => {
                  if (mountedRef.current) {
                    console.log('[EventSource] Retrying after rate limit');
                    connectionLockRef.current = false;
                    connect();
                  }
                }, retryAfter * 1000);
                return;
              }
              break;
            }

            case EventSourceErrorType.NETWORK:
            case EventSourceErrorType.SERVER:
            case EventSourceErrorType.UNKNOWN: {
              // Implement exponential backoff for these errors
              const shouldRetry = retryCountRef.current < maxRetries;
              if (shouldRetry) {
                const backoffTime = getBackoffDelay(retryCountRef.current);
                retryCountRef.current++;
                
                reconnectTimeoutRef.current = window.setTimeout(() => {
                  if (mountedRef.current) {
                    console.log(`[EventSource] Attempting reconnection (${retryCountRef.current}/${maxRetries})`);
                    connectionLockRef.current = false;
                    connect();
                  }
                }, backoffTime);
              } else {
                console.error('[EventSource] Max retry attempts reached, giving up');
                connectionLockRef.current = false;
              }
              break;
            }
          }
        };
      } catch (error) {
        if (!mountedRef.current) return;

        console.error('[EventSource] Error creating connection:', error);
        setIsConnected(false);
        const connectionError = createError(
          EventSourceErrorType.UNKNOWN,
          'Error creating connection'
        );
        onError?.(connectionError);
        connectionLockRef.current = false;
      }
    };

    connect();

    return () => {
      mountedRef.current = false;
      cleanup();
    };
  }, [path, isAuthenticated, onMessage, onError, maxRetries, reconnectInterval, cleanup, getBackoffDelay, processPendingUpdates]);

  // Reset auth error flag when auth state changes
  useEffect(() => {
    if (isAuthenticated) {
      authErrorRef.current = false;
    }
  }, [isAuthenticated]);

  return {
    isConnected,
    reconnect: useCallback(() => {
      retryCountRef.current = 0; // Reset retry count
      authErrorRef.current = false; // Reset auth error flag
      cleanup();
      if (mountedRef.current) {
        console.log('[EventSource] Manually reconnecting');
        connectionLockRef.current = false;
        eventSourceRef.current = null; // Force new connection
      }
    }, [cleanup])
  };
};
