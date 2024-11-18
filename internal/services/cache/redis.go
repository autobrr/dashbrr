// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	ErrKeyNotFound = errors.New("cache: key not found")
	ErrClosed      = errors.New("cache: store is closed")
)

const (
	PrefixSession  = "session:"
	PrefixHealth   = "health:"
	PrefixVersion  = "version:"
	PrefixRate     = "rate:"
	DefaultTimeout = 30 * time.Second
	RetryAttempts  = 2
	RetryDelay     = 50 * time.Millisecond

	// Cache durations
	DefaultTTL  = 15 * time.Minute
	HealthTTL   = 30 * time.Minute
	StatsTTL    = 5 * time.Minute
	SessionsTTL = 1 * time.Minute

	CleanupInterval = 1 * time.Minute // Increased to reduce cleanup frequency
)

// RedisStore represents a Redis cache instance with local memory cache
type RedisStore struct {
	client *redis.Client
	cfg    Config
	closed bool
	mu     sync.RWMutex
}

func NewRedisStore(cfg Config) Store {
	opts := getRedisOptions(cfg.RedisAddr)

	client := redis.NewClient(opts)

	store := &RedisStore{
		client: client,
		cfg:    cfg,
	}

	return store
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Get retrieves a value from cache with local cache first
func (s *RedisStore) Get(ctx context.Context, key string, value interface{}) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	//// Try local cache first
	//if data, ok := s.getFromLocalCache(key); ok {
	//	if err := json.Unmarshal(data, value); err != nil {
	//		log.Error().Err(err).Str("key", key).Msg("Failed to unmarshal local cached value")
	//	} else {
	//		return nil
	//	}
	//}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			data, err := s.client.Get(timeoutCtx, key).Bytes()
			cancel()

			if err == nil {
				// Store in local cache with same TTL as Redis
				ttl := s.client.TTL(ctx, key).Val()
				if ttl < 0 {
					if strings.HasPrefix(key, PrefixHealth) {
						ttl = HealthTTL
					} else if strings.HasPrefix(key, "sessions:") {
						ttl = SessionsTTL
					} else if strings.HasPrefix(key, "stats:") {
						ttl = StatsTTL
					} else {
						ttl = DefaultTTL
					}
				}
				//s.setInLocalCache(key, data, ttl)
				return json.Unmarshal(data, value)
			}

			lastErr = err
			if errors.Is(err, redis.Nil) {
				break
			}

			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	if errors.Is(lastErr, redis.Nil) {
		return ErrKeyNotFound
	}

	return lastErr
}

// Set stores a value in both Redis and local cache
func (s *RedisStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	if expiration == 0 {
		if strings.HasPrefix(key, PrefixHealth) {
			expiration = HealthTTL
		} else if strings.HasPrefix(key, "sessions:") {
			expiration = SessionsTTL
		} else if strings.HasPrefix(key, "stats:") {
			expiration = StatsTTL
		} else {
			expiration = DefaultTTL
		}
	}

	data, err := json.Marshal(value)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to marshal value for cache")
		return err
	}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := s.client.Set(timeoutCtx, key, data, expiration).Err()
			cancel()

			if err == nil {
				//s.setInLocalCache(key, data, expiration)
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	return lastErr
}

// Delete removes a value from both Redis and local cache
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	//// Remove from local cache immediately
	//s.local.Lock()
	//delete(s.local.items, key)
	//s.local.Unlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := s.client.Del(timeoutCtx, key).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	return lastErr
}

// Rate limiting methods
func (s *RedisStore) Increment(ctx context.Context, key string, timestamp int64) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			member := strconv.FormatInt(timestamp, 10)
			err := s.client.ZAdd(timeoutCtx, key, redis.Z{
				Score:  float64(timestamp),
				Member: member,
			}).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

func (s *RedisStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := s.client.ZRemRangeByScore(timeoutCtx, key, "-inf", "("+strconv.FormatInt(windowStart, 10)).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

func (s *RedisStore) GetCount(ctx context.Context, key string) (int64, error) {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return 0, ErrClosed
	//}
	//s.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			count, err := s.client.ZCard(timeoutCtx, key).Result()
			cancel()

			if err == nil {
				return count, nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return 0, lastErr
}

func (s *RedisStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	if expiration == 0 {
		expiration = DefaultTTL
	}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := s.client.Expire(timeoutCtx, key, expiration).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

// Close closes the Redis connection and stops the cleanup goroutine
func (s *RedisStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.closed = true

	// Close Redis client
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

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
		PoolSize: 3,
		//MaxConnAge:   5 * time.Minute,
		//IdleTimeout:  30 * time.Second,
		ReadTimeout:  DefaultTimeout,
		WriteTimeout: DefaultTimeout,
		PoolTimeout:  DefaultTimeout,
	}

	if isDev {
		// Even smaller settings for development
		opts.PoolSize = 2
		//opts.MaxConnAge = 30 * time.Second
		//opts.IdleTimeout = 15 * time.Second
	}

	return opts
}
