// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// Config holds cache configuration options
type Config struct {
	// Redis configuration
	RedisAddr string

	// Memory cache configuration
	DataDir string // Directory for persistent storage (derived from DB path)

	// Testing flag to bypass singleton pattern
	testing bool
}

// CacheType represents the type of cache to use
type CacheType string

const (
	CacheTypeRedis  CacheType = "redis"
	CacheTypeMemory CacheType = "memory"
)

var (
	// Global cache instance
	globalCache Store
	initOnce    sync.Once
	mu          sync.RWMutex
)

// getRedisOptions returns Redis configuration optimized for the current environment
func getRedisOptions(addr string) *redis.Options {
	isDev := os.Getenv("GIN_MODE") != "release"

	// Base configuration optimized for single user
	opts := &redis.Options{
		Addr:            addr,
		MinIdleConns:    1,
		MaxRetries:      RetryAttempts,
		MinRetryBackoff: RetryDelay,
		MaxRetryBackoff: time.Second,
		// Reduced pool size for single user scenario
		PoolSize:     3,
		MaxConnAge:   5 * time.Minute,
		IdleTimeout:  30 * time.Second,
		ReadTimeout:  DefaultTimeout,
		WriteTimeout: DefaultTimeout,
		PoolTimeout:  DefaultTimeout,
	}

	if isDev {
		// Even smaller settings for development
		opts.PoolSize = 2
		opts.MaxConnAge = 30 * time.Second
		opts.IdleTimeout = 15 * time.Second
	}

	return opts
}

// getCacheType determines which cache implementation to use based on environment
func getCacheType() CacheType {
	cacheType := os.Getenv("CACHE_TYPE")
	if cacheType == "" {
		// Default to memory cache unless Redis is explicitly configured
		if os.Getenv("REDIS_HOST") != "" {
			return CacheTypeRedis
		}
		return CacheTypeMemory
	}

	switch strings.ToLower(cacheType) {
	case "redis":
		return CacheTypeRedis
	case "memory":
		return CacheTypeMemory
	default:
		log.Warn().Str("type", cacheType).Msg("Unknown cache type specified, using memory cache")
		return CacheTypeMemory
	}
}

// createCache creates a new cache instance
func createCache(ctx context.Context, cfg Config) (Store, error) {
	cacheType := getCacheType()
	var err error

	switch cacheType {
	case CacheTypeRedis:
		if cfg.RedisAddr == "" {
			return NewMemoryStore(ctx, cfg.DataDir), nil
		}

		isDev := os.Getenv("GIN_MODE") != "release"
		opts := getRedisOptions(cfg.RedisAddr)

		timeout := DefaultTimeout
		if isDev {
			timeout = 2 * time.Second
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		client := redis.NewClient(opts)

		err = client.Ping(timeoutCtx).Err()
		if err != nil {
			if client != nil {
				client.Close()
			}
			if os.Getenv("CACHE_TYPE") == "redis" {
				log.Error().Err(err).Str("addr", opts.Addr).Msg("Failed to connect to explicitly configured Redis, falling back to memory cache")
			}
			return NewMemoryStore(ctx, cfg.DataDir), err
		}

		storeCtx, storeCancel := context.WithCancel(ctx)
		store := &RedisStore{
			client: client,
			local: &LocalCache{
				items: make(map[string]*localCacheItem),
			},
			ctx:    storeCtx,
			cancel: storeCancel,
		}

		store.wg.Add(1)
		go func() {
			defer store.wg.Done()
			store.localCacheCleanup()
		}()

		return store, nil

	case CacheTypeMemory:
		return NewMemoryStore(ctx, cfg.DataDir), nil

	default:
		return NewMemoryStore(ctx, cfg.DataDir), nil
	}
}

// InitCache initializes a cache instance based on configuration.
// It always returns a valid cache store, falling back to memory cache if Redis fails.
func InitCache(ctx context.Context, cfg Config) (Store, error) {
	// For testing, bypass singleton pattern
	if cfg.testing {
		return createCache(ctx, cfg)
	}

	mu.RLock()
	if globalCache != nil {
		cache := globalCache
		mu.RUnlock()
		return cache, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// Double check after acquiring write lock
	if globalCache != nil {
		return globalCache, nil
	}

	var err error
	initOnce.Do(func() {
		globalCache, err = createCache(ctx, cfg)
	})

	return globalCache, err
}
