// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"os"
	"strings"
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
}

// CacheType represents the type of cache to use
type CacheType string

const (
	CacheTypeRedis  CacheType = "redis"
	CacheTypeMemory CacheType = "memory"
)

// getRedisOptions returns Redis configuration optimized for the current environment
func getRedisOptions(addr string) *redis.Options {
	isDev := os.Getenv("GIN_MODE") != "release"

	// Base configuration
	opts := &redis.Options{
		Addr:            addr,
		MinIdleConns:    2,
		MaxRetries:      RetryAttempts,
		MinRetryBackoff: RetryDelay,
		MaxRetryBackoff: time.Second,
	}

	if isDev {
		// Development-optimized settings
		opts.PoolSize = 5
		opts.MaxConnAge = 30 * time.Second
		opts.ReadTimeout = 2 * time.Second
		opts.WriteTimeout = 2 * time.Second
		opts.PoolTimeout = 2 * time.Second
		opts.IdleTimeout = 30 * time.Second
	} else {
		// Production settings
		opts.PoolSize = 10
		opts.MaxConnAge = 5 * time.Minute
		opts.ReadTimeout = DefaultTimeout
		opts.WriteTimeout = DefaultTimeout
		opts.PoolTimeout = DefaultTimeout * 2
		opts.IdleTimeout = time.Minute
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

// InitCache initializes a cache instance based on configuration.
// It always returns a valid cache store, falling back to memory cache if Redis fails.
func InitCache(cfg Config) (Store, error) {
	cacheType := getCacheType()

	switch cacheType {
	case CacheTypeRedis:
		// Only attempt Redis connection if Redis address is configured
		if cfg.RedisAddr == "" {
			// Silently fall back to memory cache when Redis isn't configured
			return NewMemoryStore(cfg.DataDir), nil
		}

		isDev := os.Getenv("GIN_MODE") != "release"
		opts := getRedisOptions(cfg.RedisAddr)

		// Create context with shorter timeout for development
		timeout := DefaultTimeout
		if isDev {
			timeout = 2 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		client := redis.NewClient(opts)
		err := client.Ping(ctx).Err()

		if err != nil {
			if client != nil {
				client.Close()
			}
			if os.Getenv("CACHE_TYPE") == "redis" {
				// Only log error if Redis was explicitly requested
				log.Error().Err(err).Str("addr", opts.Addr).Msg("Failed to connect to explicitly configured Redis, falling back to memory cache")
			}
			return NewMemoryStore(cfg.DataDir), err
		}

		// Initialize Redis cache store
		store, err := NewCache(opts.Addr)
		if err != nil {
			if os.Getenv("CACHE_TYPE") == "redis" {
				// Only log error if Redis was explicitly requested
				log.Error().Err(err).Msg("Failed to initialize explicitly configured Redis cache, falling back to memory cache")
			}
			return NewMemoryStore(cfg.DataDir), err
		}
		return store, nil

	case CacheTypeMemory:
		return NewMemoryStore(cfg.DataDir), nil

	default:
		// This shouldn't happen due to getCacheType's default
		return NewMemoryStore(cfg.DataDir), nil
	}
}
