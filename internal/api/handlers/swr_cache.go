// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"golang.org/x/sync/singleflight"
)

const swrStaleSuffix = ":stale"

type SWRCacheOptions[T any] struct {
	Store          cache.Store
	Key            string
	FreshTTL       time.Duration
	StaleTTL       time.Duration
	CircuitBreaker *resilience.CircuitBreaker
	Fetch          func() (T, error)

	// Optional stampede protection. Only applies on cache miss.
	Singleflight    *singleflight.Group
	SingleflightKey string
}

// FetchWithSWRCache implements a stale-while-revalidate pattern:
// - return cached value if present
// - if circuit is open, return stale value if present
// - otherwise fetch (with backoff retries), cache, and return
// - on fetch failure, return stale value if present
func FetchWithSWRCache[T any](ctx context.Context, opts SWRCacheOptions[T]) (T, error) {
	var zero T
	if opts.Store == nil {
		return zero, fmt.Errorf("cache store is nil")
	}
	if opts.Key == "" {
		return zero, fmt.Errorf("cache key is required")
	}
	if opts.Fetch == nil {
		return zero, fmt.Errorf("fetch function is required")
	}

	var cached T
	if err := opts.Store.Get(ctx, opts.Key, &cached); err == nil {
		return cached, nil
	}

	fetchAndCache := func() (T, error) {
		staleKey := opts.Key + swrStaleSuffix

		if opts.CircuitBreaker != nil && opts.CircuitBreaker.IsOpen() {
			if opts.StaleTTL > 0 {
				var stale T
				if err := opts.Store.Get(ctx, staleKey, &stale); err == nil {
					return stale, nil
				}
			}
			return zero, fmt.Errorf("circuit breaker is open")
		}

		var fresh T
		fetchErr := resilience.RetryWithBackoff(ctx, func() error {
			var err error
			fresh, err = opts.Fetch()
			return err
		})

		if fetchErr != nil {
			if opts.CircuitBreaker != nil {
				opts.CircuitBreaker.RecordFailure()
			}
			if opts.StaleTTL > 0 {
				var stale T
				if err := opts.Store.Get(ctx, staleKey, &stale); err == nil {
					return stale, nil
				}
			}
			return zero, fetchErr
		}

		if opts.CircuitBreaker != nil {
			opts.CircuitBreaker.RecordSuccess()
		}

		// Cache fresh; also store a longer-lived stale copy to serve on transient failures.
		if err := opts.Store.Set(ctx, opts.Key, fresh, opts.FreshTTL); err == nil && opts.StaleTTL > 0 {
			_ = opts.Store.Set(ctx, staleKey, fresh, opts.StaleTTL)
		}

		return fresh, nil
	}

	if opts.Singleflight != nil {
		key := opts.SingleflightKey
		if key == "" {
			key = opts.Key
		}

		v, err, _ := opts.Singleflight.Do(key, func() (any, error) {
			// Another request may have filled the cache while we were waiting.
			var rechecked T
			if err := opts.Store.Get(ctx, opts.Key, &rechecked); err == nil {
				return rechecked, nil
			}

			val, err := fetchAndCache()
			if err != nil {
				return nil, err
			}
			return val, nil
		})
		if err != nil {
			return zero, err
		}
		return v.(T), nil
	}

	return fetchAndCache()
}

func DeleteSWRCacheKeys(ctx context.Context, store cache.Store, key string) error {
	if store == nil {
		return fmt.Errorf("cache store is nil")
	}
	if key == "" {
		return fmt.Errorf("cache key is required")
	}
	err := store.Delete(ctx, key)
	_ = store.Delete(ctx, key+swrStaleSuffix)
	return err
}
