import { useEffect, useRef, useState, useCallback } from 'react';
import { useConfiguration } from '../contexts/useConfiguration';

interface PollingOptions<T> {
  interval?: number;
  enableCache?: boolean;
  cacheDuration?: number;
  initialData?: T | null;
  debounce?: boolean;
  debounceDelay?: number;
}

interface CacheItem<T> {
  data: T;
  timestamp: number;
}

const cache = new Map<string, CacheItem<unknown>>();

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
  } = options;

  const [data, setData] = useState<T | null>(initialData);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { configurations } = useConfiguration();

  const timeoutRef = useRef<NodeJS.Timeout>();
  const debounceTimeoutRef = useRef<NodeJS.Timeout>();

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
      const result = await fetchFn();
      
      // Update cache if enabled
      if (enableCache) {
        cache.set(cacheKey, {
          data: result,
          timestamp: Date.now(),
        });
      }

      setData(result);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('An error occurred'));
    } finally {
      setIsLoading(false);
    }
  }, [enableCache, cacheDuration, cacheKey, fetchFn]);

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

    // Initial fetch
    debouncedFetchData();

    // Set up polling
    timeoutRef.current = setInterval(debouncedFetchData, interval);

    return () => {
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
    debouncedFetchData();
  }, [debouncedFetchData]);

  return { data, error, isLoading, refresh };
}
