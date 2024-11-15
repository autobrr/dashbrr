// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
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
	local  *LocalCache
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // Added WaitGroup for graceful shutdown
	closed bool
	mu     sync.RWMutex
}

// LocalCache provides in-memory caching to reduce Redis hits
type LocalCache struct {
	sync.RWMutex
	items map[string]*localCacheItem
}

type localCacheItem struct {
	value      []byte
	expiration time.Time
}

// Get retrieves a value from cache with local cache first
func (s *RedisStore) Get(ctx context.Context, key string, value interface{}) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	// Try local cache first
	if data, ok := s.getFromLocalCache(key); ok {
		if err := json.Unmarshal(data, value); err != nil {
			log.Error().Err(err).Str("key", key).Msg("Failed to unmarshal local cached value")
		} else {
			return nil
		}
	}

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
				s.setInLocalCache(key, data, ttl)
				return json.Unmarshal(data, value)
			}

			lastErr = err
			if err == redis.Nil {
				break
			}

			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	if lastErr == redis.Nil {
		return ErrKeyNotFound
	}
	return lastErr
}

// Set stores a value in both Redis and local cache
func (s *RedisStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

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
				s.setInLocalCache(key, data, expiration)
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
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	// Remove from local cache immediately
	s.local.Lock()
	delete(s.local.items, key)
	s.local.Unlock()

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
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			member := strconv.FormatInt(timestamp, 10)
			err := s.client.ZAdd(timeoutCtx, key, &redis.Z{
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
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

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
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, ErrClosed
	}
	s.mu.RUnlock()

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
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

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

// Local cache methods
func (s *RedisStore) getFromLocalCache(key string) ([]byte, bool) {
	s.local.RLock()
	defer s.local.RUnlock()

	if item, exists := s.local.items[key]; exists {
		if time.Now().Before(item.expiration) {
			return item.value, true
		}
		delete(s.local.items, key)
	}
	return nil, false
}

func (s *RedisStore) setInLocalCache(key string, value []byte, ttl time.Duration) {
	s.local.Lock()
	defer s.local.Unlock()

	s.local.items[key] = &localCacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

func (s *RedisStore) localCacheCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				s.local.Lock()
				defer s.local.Unlock()

				now := time.Now()
				for key, item := range s.local.items {
					if now.After(item.expiration) {
						delete(s.local.items, key)
					}
				}
			}()
		case <-s.ctx.Done():
			return
		}
	}
}

// Close closes the Redis connection and stops the cleanup goroutine
func (s *RedisStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.mu.Unlock()

	// Cancel context to stop cleanup goroutine
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for cleanup goroutine to finish
	s.wg.Wait()

	// Clear local cache
	func() {
		s.local.Lock()
		defer s.local.Unlock()
		s.local.items = make(map[string]*localCacheItem)
	}()

	// Close Redis client
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
