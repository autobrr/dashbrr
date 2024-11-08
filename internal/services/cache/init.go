// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
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

// InitCache initializes a Redis cache instance with environment-specific configuration
func InitCache() (Store, error) {
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
			// In development, log warning but continue with degraded functionality
			log.Warn().Err(err).Str("addr", opts.Addr).Msg("Redis connection failed, some features may be degraded")
			return NewCache(opts.Addr)
		}
		if client != nil {
			client.Close()
		}
		return nil, err
	}

	// Initialize cache store with the configured client
	return NewCache(opts.Addr)
}
