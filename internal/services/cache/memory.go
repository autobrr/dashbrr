// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MemoryStore implements Store interface using in-memory storage
type MemoryStore struct {
	local  *LocalCache
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
	mu     sync.RWMutex

	// Additional maps for rate limiting functionality
	rateLimits sync.Map // map[string]*rateWindow
}

type rateWindow struct {
	sync.RWMutex
	timestamps map[string]int64
}

// NewMemoryStore creates a new in-memory cache instance
func NewMemoryStore() Store {
	ctx, cancel := context.WithCancel(context.Background())

	store := &MemoryStore{
		local: &LocalCache{
			items: make(map[string]*localCacheItem),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Start cleanup goroutine
	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		store.localCacheCleanup()
	}()

	return store
}

// Get retrieves a value from cache
func (s *MemoryStore) Get(ctx context.Context, key string, value interface{}) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	s.local.RLock()
	item, exists := s.local.items[key]
	if exists && time.Now().Before(item.expiration) {
		s.local.RUnlock()
		return json.Unmarshal(item.value, value)
	}
	if exists {
		delete(s.local.items, key)
	}
	s.local.RUnlock()

	return ErrKeyNotFound
}

// Set stores a value in cache
func (s *MemoryStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	if expiration == 0 {
		expiration = DefaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to marshal value for cache")
		return err
	}

	s.local.Lock()
	s.local.items[key] = &localCacheItem{
		value:      data,
		expiration: time.Now().Add(expiration),
	}
	s.local.Unlock()

	return nil
}

// Delete removes a value from cache
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	s.local.Lock()
	delete(s.local.items, key)
	s.local.Unlock()

	return nil
}

// Increment adds a timestamp to the rate limit window
func (s *MemoryStore) Increment(ctx context.Context, key string, timestamp int64) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	window, _ := s.rateLimits.LoadOrStore(key, &rateWindow{
		timestamps: make(map[string]int64),
	})
	w := window.(*rateWindow)

	w.Lock()
	w.timestamps[strconv.FormatInt(timestamp, 10)] = timestamp
	w.Unlock()

	return nil
}

// CleanAndCount removes old timestamps and returns the count of remaining ones
func (s *MemoryStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	if window, ok := s.rateLimits.Load(key); ok {
		w := window.(*rateWindow)
		w.Lock()
		for ts, timestamp := range w.timestamps {
			if timestamp < windowStart {
				delete(w.timestamps, ts)
			}
		}
		w.Unlock()
	}

	return nil
}

// GetCount returns the number of timestamps in the current window
func (s *MemoryStore) GetCount(ctx context.Context, key string) (int64, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, ErrKeyNotFound
	}
	s.mu.RUnlock()

	if window, ok := s.rateLimits.Load(key); ok {
		w := window.(*rateWindow)
		w.RLock()
		count := int64(len(w.timestamps))
		w.RUnlock()
		return count, nil
	}

	return 0, nil
}

// Expire updates the expiration time for a key
func (s *MemoryStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrKeyNotFound
	}
	s.mu.RUnlock()

	s.local.Lock()
	if item, exists := s.local.items[key]; exists {
		item.expiration = time.Now().Add(expiration)
	}
	s.local.Unlock()

	return nil
}

// Close cleans up resources
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrKeyNotFound
	}
	s.closed = true
	s.mu.Unlock()

	s.cancel()
	s.wg.Wait()

	s.local.Lock()
	s.local.items = make(map[string]*localCacheItem)
	s.local.Unlock()

	return nil
}

func (s *MemoryStore) localCacheCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()

			// Cleanup main cache
			s.local.Lock()
			for key, item := range s.local.items {
				if now.After(item.expiration) {
					delete(s.local.items, key)
				}
			}
			s.local.Unlock()

			// Cleanup rate limiting windows older than 24 hours
			windowStart := time.Now().Add(-24 * time.Hour).Unix()
			s.rateLimits.Range(func(key, value interface{}) bool {
				w := value.(*rateWindow)
				w.Lock()
				for ts, timestamp := range w.timestamps {
					if timestamp < windowStart {
						delete(w.timestamps, ts)
					}
				}
				w.Unlock()
				return true
			})

		case <-s.ctx.Done():
			return
		}
	}
}
