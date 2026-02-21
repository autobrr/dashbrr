// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"sync"
)

// Config holds cache configuration options.
type Config struct {
	// Directory for persistent storage (derived from DB path).
	DataDir string

	// Testing flag to bypass singleton pattern.
	testing bool
}

var (
	// Global cache instance.
	globalCache Store
	initOnce    sync.Once
	mu          sync.RWMutex
)

func createCache(ctx context.Context, cfg Config) Store {
	return NewMemoryStore(ctx, cfg.DataDir)
}

// InitCache initializes the global in-memory cache instance.
func InitCache(ctx context.Context, cfg Config) (Store, error) {
	// For testing, bypass singleton pattern.
	if cfg.testing {
		return createCache(ctx, cfg), nil
	}

	mu.RLock()
	if globalCache != nil {
		cache := globalCache
		mu.RUnlock()
		return cache, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// Double check after acquiring write lock.
	if globalCache != nil {
		return globalCache, nil
	}

	initOnce.Do(func() {
		globalCache = createCache(ctx, cfg)
	})

	return globalCache, nil
}
