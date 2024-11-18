// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// Config holds cache configuration options
type Config struct {
	// Cache type
	Type CacheType

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

	switch cacheType {
	case CacheTypeRedis:
		if cfg.RedisAddr == "" {
			return NewMemoryStore(ctx, cfg.DataDir), nil
		}
		return NewRedisStore(cfg), nil

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
