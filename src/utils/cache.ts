interface CacheEntry<T> {
  data: T;
  timestamp: number;
  staleAt: number;
  isStale: boolean;
  lastModified?: string;
  etag?: string;
}

interface CacheConfig {
  ttl: number; // Time to live in milliseconds
  staleWhileRevalidate: number; // Additional time to serve stale content while revalidating
  maxEntries?: number; // Maximum number of entries to store in cache
}

// Cache prefixes matching backend
const CACHE_PREFIXES = {
  STATS: 'stats_',
  HEALTH: 'health_',
  AUTOBRR: {
    STATS: 'autobrr:stats:',
    IRC: 'autobrr:irc:'
  },
  PLEX: {
    SESSIONS: 'plex:sessions:'
  },
  OVERSEERR: {
    STATS: 'overseerr:stats:'
  },
  MAINTAINERR: {
    COLLECTIONS: 'maintainerr:collections:'
  }
};

// Keys to preserve (never clean up)
const PRESERVED_KEYS = [
  'access_token',
  'id_token',
  'auth_type',
  'dashbrr-service-order',
];

// Cache times for different types of data
const HEALTH_TTL = 300000;                // 5 minutes for health checks
const DEFAULT_STATS_TTL = 30000;          // 30 seconds default for stats
const DEFAULT_MAX_ENTRIES = 1000;

// Service-specific cache configurations
const SERVICE_CACHE_CONFIGS: Record<string, CacheConfig> = {
  // Health checks use 5-minute TTL
  [CACHE_PREFIXES.HEALTH]: {
    ttl: HEALTH_TTL,
    staleWhileRevalidate: 60000, // 1 minute stale time for health
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  // Plex sessions need very short TTL
  [CACHE_PREFIXES.PLEX.SESSIONS]: {
    ttl: 10000, // 10 seconds
    staleWhileRevalidate: 5000,
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  // Autobrr stats and IRC status
  [CACHE_PREFIXES.AUTOBRR.STATS]: {
    ttl: 30000, // 30 seconds
    staleWhileRevalidate: 15000,
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  [CACHE_PREFIXES.AUTOBRR.IRC]: {
    ttl: 30000, // 30 seconds
    staleWhileRevalidate: 15000,
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  // Overseerr stats
  [CACHE_PREFIXES.OVERSEERR.STATS]: {
    ttl: 60000, // 1 minute
    staleWhileRevalidate: 30000,
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  // Maintainerr collections
  [CACHE_PREFIXES.MAINTAINERR.COLLECTIONS]: {
    ttl: 60000, // 1 minute
    staleWhileRevalidate: 30000,
    maxEntries: DEFAULT_MAX_ENTRIES
  },
  // Default stats configuration
  [CACHE_PREFIXES.STATS]: {
    ttl: DEFAULT_STATS_TTL,
    staleWhileRevalidate: 15000, // 15 seconds
    maxEntries: DEFAULT_MAX_ENTRIES
  }
};

export class Cache {
  private static instance: Cache;
  private config: CacheConfig = {
    ttl: DEFAULT_STATS_TTL,
    staleWhileRevalidate: 15000,
    maxEntries: DEFAULT_MAX_ENTRIES,
  };
  private memoryCache: Map<string, CacheEntry<unknown>> = new Map();

  private constructor() {
    this.loadFromLocalStorage();
    setInterval(() => this.cleanup(), 60000); // Run cleanup every minute
  }

  static getInstance(): Cache {
    if (!Cache.instance) {
      Cache.instance = new Cache();
    }
    return Cache.instance;
  }

  setConfig(config: Partial<CacheConfig>) {
    this.config = { ...this.config, ...config };
  }

  private getConfigForKey(key: string): CacheConfig {
    // Find matching service config based on key prefix
    for (const [prefix, config] of Object.entries(SERVICE_CACHE_CONFIGS)) {
      if (key.includes(prefix)) {
        return config;
      }
    }
    return this.config;
  }

  private shouldCacheKey(key: string): boolean {
    // Never cache preserved keys
    if (PRESERVED_KEYS.includes(key)) return false;
    
    // Never cache service collapse states
    if (key.includes('dashbrr-service-') && key.endsWith('-collapsed')) return false;

    return true;
  }

  private loadFromLocalStorage() {
    try {
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key && this.shouldCacheKey(key)) {
          const value = localStorage.getItem(key);
          if (value) {
            try {
              const entry = JSON.parse(value) as CacheEntry<unknown>;
              // Only load if not expired
              if (Date.now() <= entry.staleAt + this.getConfigForKey(key).staleWhileRevalidate) {
                this.memoryCache.set(key, entry);
              } else {
                localStorage.removeItem(key);
              }
            } catch (error) {
              console.debug('Skipping non-JSON localStorage item:', key, error);
            }
          }
        }
      }
    } catch (error) {
      console.error('Error loading cache from localStorage:', error);
    }
  }

  private persistToLocalStorage(key: string, entry: CacheEntry<unknown>) {
    if (!this.shouldCacheKey(key)) return;
    
    try {
      localStorage.setItem(key, JSON.stringify(entry));
    } catch (error: unknown) {
      console.error('Error persisting to localStorage:', error);
      if (error instanceof Error && error.name === 'QuotaExceededError') {
        this.cleanup(true);
        try {
          localStorage.setItem(key, JSON.stringify(entry));
        } catch (retryError) {
          console.error('Failed to persist to localStorage after cleanup:', retryError);
        }
      }
    }
  }

  set<T>(key: string, data: T, metadata?: { lastModified?: string; etag?: string }): void {
    if (!this.shouldCacheKey(key)) return;

    const now = Date.now();
    const config = this.getConfigForKey(key);
    const entry: CacheEntry<T> = {
      data,
      timestamp: now,
      staleAt: now + config.ttl,
      isStale: false,
      ...metadata
    };

    this.memoryCache.set(key, entry as CacheEntry<unknown>);
    this.persistToLocalStorage(key, entry);
  }

  get<T>(key: string): { data: T | null; isStale: boolean; metadata?: { lastModified?: string; etag?: string } } {
    if (!this.shouldCacheKey(key)) {
      return { data: null, isStale: false };
    }

    // First check memory cache
    const entry = this.memoryCache.get(key) as CacheEntry<T> | undefined;
    if (!entry) {
      // If not in memory, try localStorage
      const stored = localStorage.getItem(key);
      if (stored) {
        try {
          const storedEntry = JSON.parse(stored) as CacheEntry<T>;
          this.memoryCache.set(key, storedEntry as CacheEntry<unknown>);
          return this.evaluateEntry(storedEntry, key);
        } catch (error) {
          console.debug('Failed to parse stored cache entry:', key, error);
          return { data: null, isStale: false };
        }
      }
      return { data: null, isStale: false };
    }

    return this.evaluateEntry(entry, key);
  }

  private evaluateEntry<T>(entry: CacheEntry<T>, key: string) {
    const now = Date.now();
    const config = this.getConfigForKey(key);

    // If the data is fresh, return it
    if (now <= entry.staleAt) {
      return { 
        data: entry.data, 
        isStale: false,
        metadata: {
          lastModified: entry.lastModified,
          etag: entry.etag
        }
      };
    }

    // If the data is within the stale window, mark it as stale but still return it
    if (now <= entry.staleAt + config.staleWhileRevalidate) {
      return { 
        data: entry.data, 
        isStale: true,
        metadata: {
          lastModified: entry.lastModified,
          etag: entry.etag
        }
      };
    }

    // Data is too old, remove it
    this.remove(key);
    return { data: null, isStale: false };
  }

  remove(key: string): void {
    if (!this.shouldCacheKey(key)) return;
    
    this.memoryCache.delete(key);
    localStorage.removeItem(key);
  }

  clear(): void {
    // Only clear cacheable items
    for (const key of Array.from(this.memoryCache.keys())) {
      if (this.shouldCacheKey(key)) {
        this.remove(key);
      }
    }
    
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i);
      if (key && this.shouldCacheKey(key)) {
        localStorage.removeItem(key);
      }
    }
  }

  private cleanup(force: boolean = false) {
    const now = Date.now();
    const entries = Array.from(this.memoryCache.entries());
    
    for (const [key, entry] of entries) {
      if (!this.shouldCacheKey(key)) continue;
      
      const config = this.getConfigForKey(key);
      if (
        now > entry.staleAt + config.staleWhileRevalidate ||
        (force && this.memoryCache.size > config.maxEntries! * 0.75)
      ) {
        this.remove(key);
      }
    }
  }

  getKeysByPrefix(prefix: string): string[] {
    return Array.from(this.memoryCache.keys())
      .filter(key => this.shouldCacheKey(key) && key.startsWith(prefix));
  }

  removeByPrefix(prefix: string): void {
    const keys = this.getKeysByPrefix(prefix);
    keys.forEach(key => this.remove(key));
  }
}

export const cache = Cache.getInstance();
export { CACHE_PREFIXES };
