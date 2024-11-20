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

// Get retrieves a value from cache
func (s *RedisStore) Get(ctx context.Context, key string, value interface{}) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrKeyNotFound
		}

		return err
	}

	return json.Unmarshal(data, value)
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

	err = s.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return err
	}

	return nil
}

// Delete removes a value from both Redis and local cache
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	err := s.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}

// Rate limiting methods
func (s *RedisStore) Increment(ctx context.Context, key string, timestamp int64) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	member := strconv.FormatInt(timestamp, 10)
	err := s.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(timestamp),
		Member: member,
	}).Err()
	if err != nil {
		return err
	}

	return nil
}

func (s *RedisStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return ErrClosed
	//}
	//s.mu.RUnlock()

	err := s.client.ZRemRangeByScore(ctx, key, "-inf", "("+strconv.FormatInt(windowStart, 10)).Err()
	if err != nil {
		return err
	}

	return nil
}

func (s *RedisStore) GetCount(ctx context.Context, key string) (int64, error) {
	//s.mu.RLock()
	//if s.closed {
	//	s.mu.RUnlock()
	//	return 0, ErrClosed
	//}
	//s.mu.RUnlock()
	count, err := s.client.ZCard(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return count, nil
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

	err := s.client.Expire(ctx, key, expiration).Err()
	if err != nil {
		return err
	}

	return nil
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
		Addr: addr,
		//MinIdleConns: 1,
		//MaxRetries:      RetryAttempts,
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
