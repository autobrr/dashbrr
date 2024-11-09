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
		// Default to memory cache if Redis host is not set
		if os.Getenv("REDIS_HOST") == "" {
			return CacheTypeMemory
		}
		return CacheTypeRedis
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

// InitCache initializes a cache instance based on environment configuration
func InitCache() (Store, error) {
	cacheType := getCacheType()

	log.Debug().Str("type", string(cacheType)).Msg("Initializing cache")

	switch cacheType {
	case CacheTypeRedis:
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
			if isDev {
				// In development, log warning and fallback to memory cache
				log.Warn().Err(err).Str("addr", opts.Addr).Msg("Redis connection failed, falling back to memory cache")
				return NewMemoryStore(), nil
			}
			if client != nil {
				client.Close()
			}
			return nil, err
		}

		// Initialize Redis cache store
		return NewCache(opts.Addr)

	case CacheTypeMemory:
		log.Debug().Msg("Initializing memory cache")
		return NewMemoryStore(), nil

	default:
		// This shouldn't happen due to getCacheType's default
		return NewMemoryStore(), nil
	}
}
