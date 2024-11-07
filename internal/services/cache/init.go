// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

var (
	instance     *RedisStore
	initOnce     sync.Once
	isInitiating int32
)

// GetRedisAddress returns the Redis connection address from environment variables
func GetRedisAddress() string {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")

	if host == "" {
		host = "localhost"
		log.Debug().Msg("Using default Redis host: localhost")
	}
	if port == "" {
		port = "6379"
		log.Debug().Msg("Using default Redis port: 6379")
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Debug().Str("addr", addr).Msg("Redis address configured")
	return addr
}

// InitCache initializes and returns a new Cache instance
func InitCache() (Store, error) {
	// If we're already initializing, return the existing instance or wait for initialization
	if !atomic.CompareAndSwapInt32(&isInitiating, 0, 1) {
		log.Debug().Msg("Cache initialization already in progress, waiting...")
		// Wait for initialization to complete and return existing instance
		for atomic.LoadInt32(&isInitiating) == 1 {
			if instance != nil {
				return instance, nil
			}
		}
		return instance, nil
	}

	defer atomic.StoreInt32(&isInitiating, 0)

	var initErr error
	initOnce.Do(func() {
		if instance != nil {
			instance.mu.RLock()
			closed := instance.closed
			instance.mu.RUnlock()

			if !closed {
				return
			}

			// If the instance is closed, clean it up properly
			if err := instance.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close existing Redis cache instance")
			}
			instance = nil
		}

		// Create new instance
		log.Debug().Msg("Initializing Redis cache")
		addr := GetRedisAddress()
		store, err := NewCache(addr)
		if err != nil {
			initErr = err
			return
		}

		// Type assertion since we know it's a RedisStore
		redisStore, ok := store.(*RedisStore)
		if !ok {
			initErr = fmt.Errorf("unexpected store type")
			return
		}
		instance = redisStore
	})

	if initErr != nil {
		return nil, initErr
	}

	return instance, nil
}
