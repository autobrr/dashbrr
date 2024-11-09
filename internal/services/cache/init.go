// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// CacheType represents the type of cache to use
type CacheType string

const (
	CacheTypeRedis  CacheType = "redis"
	CacheTypeMemory CacheType = "memory"
)

// getRedisOptions returns Redis configuration optimized for the current environment
func getRedisOptions() *redis.Options {
	isDev := os.Getenv("GIN_MODE") != "release"

	// Get Redis connection details from environment
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

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
		// If CACHE_TYPE is not set, use Redis only if REDIS_HOST is set
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
		log.Warn().Str("type", cacheType).Msg("Unknown cache type specified, defaulting to memory cache")
		return CacheTypeMemory
	}
}

// InitCache initializes a cache instance based on environment configuration.
// It always returns a valid cache store, falling back to memory cache if Redis fails.
func InitCache() (Store, error) {
	cacheType := getCacheType()

	log.Debug().Str("type", string(cacheType)).Msg("Initializing cache")

	switch cacheType {
	case CacheTypeRedis:
		// Only attempt Redis connection if Redis is explicitly configured
		if os.Getenv("REDIS_HOST") == "" {
			log.Debug().Msg("Redis cache type selected but REDIS_HOST not set, using memory cache")
			return NewMemoryStore(), nil
		}

		isDev := os.Getenv("GIN_MODE") != "release"
		opts := getRedisOptions()

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
			// Only show Redis connection error if Redis was explicitly configured
			log.Error().Err(err).Str("addr", opts.Addr).Msg("Failed to connect to Redis, using memory cache")
			return NewMemoryStore(), err
		}

		// Initialize Redis cache store
		store, err := NewCache(opts.Addr)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize Redis cache, using memory cache")
			return NewMemoryStore(), err
		}
		return store, nil

	case CacheTypeMemory:
		log.Debug().Msg("Initializing memory cache")
		return NewMemoryStore(), nil

	default:
		// This shouldn't happen due to getCacheType's default
		return NewMemoryStore(), nil
	}
}
