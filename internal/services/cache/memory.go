// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Session persistence
	persistPath string
}

type rateWindow struct {
	sync.RWMutex
	timestamps map[string]int64
	expiration time.Time
}

type persistedItem struct {
	Value      []byte    `json:"value"`
	Expiration time.Time `json:"expiration"`
}

// NewMemoryStore creates a new in-memory cache instance
func NewMemoryStore(dataDir string) Store {
	ctx, cancel := context.WithCancel(context.Background())

	store := &MemoryStore{
		local: &LocalCache{
			items: make(map[string]*localCacheItem),
		},
		ctx:         ctx,
		cancel:      cancel,
		persistPath: filepath.Join(dataDir, "sessions.json"),
	}

	// Ensure directory exists with proper permissions
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create data directory")
	}

	// Set proper permissions on sessions file if it exists
	if _, err := os.Stat(store.persistPath); err == nil {
		if err := os.Chmod(store.persistPath, 0600); err != nil {
			log.Error().Err(err).Msg("Failed to set permissions on sessions file")
		}
	}

	// Load persisted sessions
	store.loadSessions()

	// Start cleanup goroutine
	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		store.localCacheCleanup()
	}()

	return store
}

// loadSessions loads persisted sessions from disk
func (s *MemoryStore) loadSessions() {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read persisted sessions")
		}
		return
	}

	var items map[string]persistedItem
	if err := json.Unmarshal(data, &items); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal persisted sessions")
		return
	}

	now := time.Now()
	s.local.Lock()
	for key, item := range items {
		// Only load non-expired sessions
		if now.Before(item.Expiration) {
			s.local.items[key] = &localCacheItem{
				value:      item.Value,
				expiration: item.Expiration,
			}
		}
	}
	s.local.Unlock()
}

// persistSessions saves sessions to disk
func (s *MemoryStore) persistSessions() {
	s.local.RLock()
	items := make(map[string]persistedItem)
	now := time.Now()

	for key, item := range s.local.items {
		// Only persist session data (not rate limiting or other cache items)
		if strings.HasPrefix(key, "session:") || strings.HasPrefix(key, "oidc:session:") {
			// Only persist non-expired sessions
			if now.Before(item.expiration) {
				items[key] = persistedItem{
					Value:      item.value,
					Expiration: item.expiration,
				}
			}
		}
	}
	s.local.RUnlock()

	data, err := json.Marshal(items)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal sessions for persistence")
		return
	}

	// Write to a temporary file first
	tempFile := s.persistPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		log.Error().Err(err).Msg("Failed to write temporary sessions file")
		return
	}

	// Rename temporary file to actual file (atomic operation)
	if err := os.Rename(tempFile, s.persistPath); err != nil {
		log.Error().Err(err).Msg("Failed to rename temporary sessions file")
		_ = os.Remove(tempFile) // Clean up temp file if rename failed
		return
	}
}

// Get retrieves a value from cache
func (s *MemoryStore) Get(ctx context.Context, key string, value interface{}) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
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
		return ErrClosed
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

	// Persist sessions when they're updated
	if strings.HasPrefix(key, "session:") || strings.HasPrefix(key, "oidc:session:") {
		s.persistSessions()
	}

	return nil
}

// Delete removes a value from cache
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	s.local.Lock()
	delete(s.local.items, key)
	s.local.Unlock()

	// Persist sessions when they're deleted
	if strings.HasPrefix(key, "session:") || strings.HasPrefix(key, "oidc:session:") {
		s.persistSessions()
	}

	return nil
}

// Increment adds a timestamp to the rate limit window
func (s *MemoryStore) Increment(ctx context.Context, key string, timestamp int64) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	window, _ := s.rateLimits.LoadOrStore(key, &rateWindow{
		timestamps: make(map[string]int64),
		expiration: time.Now().Add(24 * time.Hour),
	})
	w := window.(*rateWindow)

	w.Lock()
	defer w.Unlock()

	// Check if window has expired
	if time.Now().After(w.expiration) {
		w.timestamps = make(map[string]int64)
		w.expiration = time.Now().Add(24 * time.Hour)
	}

	w.timestamps[strconv.FormatInt(timestamp, 10)] = timestamp
	return nil
}

// CleanAndCount removes old timestamps and returns the count of remaining ones
func (s *MemoryStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	if window, ok := s.rateLimits.Load(key); ok {
		w := window.(*rateWindow)
		w.Lock()
		defer w.Unlock()

		// Check if window has expired
		if time.Now().After(w.expiration) {
			w.timestamps = make(map[string]int64)
			w.expiration = time.Now().Add(24 * time.Hour)
			return nil
		}

		for ts, timestamp := range w.timestamps {
			if timestamp < windowStart {
				delete(w.timestamps, ts)
			}
		}
	}

	return nil
}

// GetCount returns the number of timestamps in the current window
func (s *MemoryStore) GetCount(ctx context.Context, key string) (int64, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, ErrClosed
	}
	s.mu.RUnlock()

	if window, ok := s.rateLimits.Load(key); ok {
		w := window.(*rateWindow)
		w.RLock()
		defer w.RUnlock()

		// Check if window has expired
		if time.Now().After(w.expiration) {
			return 0, nil
		}

		return int64(len(w.timestamps)), nil
	}

	return 0, nil
}

// Expire updates the expiration time for a key
func (s *MemoryStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrClosed
	}
	s.mu.RUnlock()

	// Handle rate limit windows
	if window, ok := s.rateLimits.Load(key); ok {
		w := window.(*rateWindow)
		w.Lock()
		w.expiration = time.Now().Add(expiration)
		w.Unlock()
		return nil
	}

	// Handle regular cache items
	s.local.Lock()
	defer s.local.Unlock()

	if item, exists := s.local.items[key]; exists {
		item.expiration = time.Now().Add(expiration)
		// Persist sessions when their expiration is updated
		if strings.HasPrefix(key, "session:") || strings.HasPrefix(key, "oidc:session:") {
			s.persistSessions()
		}
	}

	return nil
}

// Close cleans up resources
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.mu.Unlock()

	s.cancel()
	s.wg.Wait()

	// Persist sessions before clearing the cache
	s.persistSessions()

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
			needsPersist := false

			// Cleanup main cache
			s.local.Lock()
			for key, item := range s.local.items {
				if now.After(item.expiration) {
					delete(s.local.items, key)
					if strings.HasPrefix(key, "session:") || strings.HasPrefix(key, "oidc:session:") {
						needsPersist = true
					}
				}
			}
			s.local.Unlock()

			// Persist sessions if any were removed
			if needsPersist {
				s.persistSessions()
			}

			// Cleanup expired rate limiting windows
			s.rateLimits.Range(func(key, value interface{}) bool {
				w := value.(*rateWindow)
				w.Lock()
				if now.After(w.expiration) {
					s.rateLimits.Delete(key)
				}
				w.Unlock()
				return true
			})

		case <-s.ctx.Done():
			return
		}
	}
}
