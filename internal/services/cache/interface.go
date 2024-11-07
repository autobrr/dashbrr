// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"time"
)

// Store defines the caching operations.
// Implementations must be safe for concurrent use.
type Store interface {
	Get(ctx context.Context, key string, value interface{}) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	Increment(ctx context.Context, key string, timestamp int64) error
	CleanAndCount(ctx context.Context, key string, windowStart int64) error
	GetCount(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Close() error
}

// Ensure RedisStore implements Store
var _ Store = (*RedisStore)(nil)
