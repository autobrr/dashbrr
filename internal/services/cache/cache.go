// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrKeyNotFound = errors.New("cache: key not found")
	ErrClosed      = errors.New("cache: store is closed")
)

const (
	PrefixSession = "session:"
	PrefixHealth  = "health:"
	PrefixVersion = "version:"
	PrefixRate    = "rate:"

	// Cache durations
	DefaultTTL  = 15 * time.Minute
	HealthTTL   = 30 * time.Minute
	StatsTTL    = 5 * time.Minute
	SessionsTTL = 1 * time.Minute

	CleanupInterval = 1 * time.Minute // Increased to reduce cleanup frequency
)

// LocalCache provides in-memory caching for store implementations.
type LocalCache struct {
	sync.RWMutex
	items map[string]*localCacheItem
}

type localCacheItem struct {
	value      []byte
	expiration time.Time
}
