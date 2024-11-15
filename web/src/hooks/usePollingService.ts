/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { useConfiguration } from '../contexts/useConfiguration';

interface PollingOptions<T> {
  interval?: number;
  enableCache?: boolean;
  cacheDuration?: number;
  initialData?: T | null;
  debounce?: boolean;
  debounceDelay?: number;
  retryAttempts?: number;
  retryDelay?: number;
}

interface CacheItem<T> {
  data: T;
  timestamp: number;
}

const cache = new Map<string, CacheItem<unknown>>();
const activeRequests = new Map<string, Promise<unknown>>();

export function usePollingService<T>(
  fetchFn: () => Promise<T>,
  cacheKey: string,
  options: PollingOptions<T> = {}
) {
  const {
    interval = 30000,
    enableCache = true,
    cacheDuration = 300000,
    initialData = null,
    debounce = true,
    debounceDelay = 1000,
    retryAttempts = 3,
    retryDelay = 1000,
  } = options;

  const [data, setData] = useState<T | null>(initialData);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { configurations } = useConfiguration();

  const timeoutRef = useRef<NodeJS.Timeout>();
  const debounceTimeoutRef = useRef<NodeJS.Timeout>();
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);

  const fetchWithRetry = useCallback(async (attempt: number = 0): Promise<T> => {
    try {
      // Check if there's already an active request for this cache key
      const activeRequest = activeRequests.get(cacheKey);
      if (activeRequest) {
        return activeRequest as Promise<T>;
      }

      // Create new request
      const request = fetchFn();
      activeRequests.set(cacheKey, request);

      const result = await request;
      activeRequests.delete(cacheKey);
      return result;
    } catch (err) {
      activeRequests.delete(cacheKey);
      
      if (attempt < retryAttempts) {
        // Exponential backoff
        const delay = retryDelay * Math.pow(2, attempt);
        await new Promise(resolve => setTimeout(resolve, delay));
        return fetchWithRetry(attempt + 1);
      }
      throw err;
    }
  }, [cacheKey, fetchFn, retryAttempts, retryDelay]);

  const fetchData = useCallback(async () => {
    try {
      // Check cache first if enabled
      if (enableCache) {
        const cachedItem = cache.get(cacheKey) as CacheItem<T> | undefined;
        if (cachedItem && Date.now() - cachedItem.timestamp < cacheDuration) {
          setData(cachedItem.data);
          return;
        }
      }

      setIsLoading(true);
      const result = await fetchWithRetry();
      
      if (!mountedRef.current) return;

      // Update cache if enabled
      if (enableCache) {
        cache.set(cacheKey, {
          data: result,
          timestamp: Date.now(),
        });
      }

      setData(result);
      setError(null);
      retryCountRef.current = 0;
    } catch (err) {
      if (!mountedRef.current) return;
      setError(err instanceof Error ? err : new Error('An error occurred'));
    } finally {
      if (mountedRef.current) {
        setIsLoading(false);
      }
    }
  }, [enableCache, cacheDuration, cacheKey, fetchWithRetry]);

  const debouncedFetchData = useCallback(() => {
    if (debounce) {
      if (debounceTimeoutRef.current) {
        clearTimeout(debounceTimeoutRef.current);
      }
      debounceTimeoutRef.current = setTimeout(fetchData, debounceDelay);
    } else {
      fetchData();
    }
  }, [debounce, debounceDelay, fetchData]);

  useEffect(() => {
    if (!configurations) return;

    mountedRef.current = true;

    // Initial fetch
    debouncedFetchData();

    // Set up polling
    timeoutRef.current = setInterval(debouncedFetchData, interval);

    return () => {
      mountedRef.current = false;
      activeRequests.forEach((_, key) => {
        activeRequests.delete(key); // Clear active requests
      });
      if (timeoutRef.current) {
        clearInterval(timeoutRef.current);
      }
      if (debounceTimeoutRef.current) {
        clearTimeout(debounceTimeoutRef.current);
      }
    };
  }, [configurations, interval, debouncedFetchData]);

  // Expose a method to force refresh data
  const refresh = useCallback(() => {
    retryCountRef.current = 0;
    debouncedFetchData();
  }, [debouncedFetchData]);

  return { data, error, isLoading, refresh };
}
