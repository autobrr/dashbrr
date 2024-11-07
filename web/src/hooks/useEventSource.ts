/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';

interface EventSourceOptions<T> {
  onMessage?: (data: T) => void;
  onError?: (error: Event) => void;
  reconnectInterval?: number;
  maxRetries?: number;
}

interface EventData {
  type: string;
  [key: string]: unknown;
}

// Constants for reconnection strategy
const MAX_BACKOFF_DELAY = 30000; // 30 seconds
const BASE_RETRY_DELAY = 5000;   // 5 seconds
const HEALTH_CHECK_INTERVAL = 60000;     // 1 minute
const MESSAGE_TIMEOUT = 300000;          // 5 minutes
const DEFAULT_MAX_RETRIES = 5;

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
  const connectionAttemptRef = useRef<boolean>(false);
  const authErrorRef = useRef<boolean>(false);

  const {
    onMessage,
    onError,
    reconnectInterval = BASE_RETRY_DELAY,
    maxRetries = DEFAULT_MAX_RETRIES
  } = options;

  // Cleanup function
  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setIsConnected(false);
    connectionAttemptRef.current = false;
  }, []);

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
    if (!path || !isAuthenticated || connectionAttemptRef.current || authErrorRef.current) {
      return;
    }

    const connect = () => {
      if (!mountedRef.current || connectionAttemptRef.current) return;

      connectionAttemptRef.current = true;

      // Get access token from localStorage
      const accessToken = localStorage.getItem('access_token');
      if (!accessToken) {
        console.error('[EventSource] No access token available for connection');
        setIsConnected(false);
        connectionAttemptRef.current = false;
        authErrorRef.current = true;
        return;
      }

      try {
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
            onMessage?.(data);
          } catch (error) {
            console.error('[EventSource] Error parsing message:', error);
          }
        };

        eventSource.addEventListener('health', (event: MessageEvent) => {
          if (!mountedRef.current) return;

          try {
            const data = JSON.parse(event.data) as T;
            lastMessageTimeRef.current = Date.now();
            onMessage?.(data);
          } catch (error) {
            console.error('[EventSource] Error parsing health event:', error);
          }
        });

        eventSource.onerror = (error) => {
          if (!mountedRef.current) return;

          console.error('[EventSource] Connection error:', error);
          setIsConnected(false);
          onError?.(error);
          cleanup();

          // Check if the error is auth-related (EventSource doesn't provide status codes)
          const authError = !isAuthenticated || !localStorage.getItem('access_token');
          if (authError) {
            console.error('[EventSource] Authentication error detected');
            authErrorRef.current = true;
            return;
          }

          // Implement exponential backoff for non-auth errors
          const shouldRetry = retryCountRef.current < maxRetries;
          if (shouldRetry) {
            const backoffTime = getBackoffDelay(retryCountRef.current);
            retryCountRef.current++;
            
            reconnectTimeoutRef.current = window.setTimeout(() => {
              if (mountedRef.current) {
                console.log(`[EventSource] Attempting reconnection (${retryCountRef.current}/${maxRetries})`);
                connectionAttemptRef.current = false;
                connect();
              }
            }, backoffTime);
          } else {
            console.error('[EventSource] Max retry attempts reached, giving up');
            connectionAttemptRef.current = false;
          }
        };
      } catch (error) {
        if (!mountedRef.current) return;

        console.error('[EventSource] Error creating connection:', error);
        setIsConnected(false);
        onError?.(new Event('error'));
        connectionAttemptRef.current = false;
      }
    };

    connect();

    return () => {
      mountedRef.current = false;
      cleanup();
    };
  }, [path, isAuthenticated, onMessage, onError, maxRetries, reconnectInterval, cleanup, getBackoffDelay]);

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
        connectionAttemptRef.current = false;
        eventSourceRef.current = null; // Force new connection
      }
    }, [cleanup])
  };
};
