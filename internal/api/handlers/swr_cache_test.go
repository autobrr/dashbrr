// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
)

type swrTestValue struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestFetchWithSWRCache_CacheHit(t *testing.T) {
	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	ctx := context.Background()

	key := "test:key"
	require.NoError(t, store.Set(ctx, key, swrTestValue{Name: "cached", N: 1}, time.Minute))

	calls := 0
	got, err := FetchWithSWRCache(ctx, SWRCacheOptions[swrTestValue]{
		Store:    store,
		Key:      key,
		FreshTTL: time.Minute,
		StaleTTL: time.Minute,
		Fetch: func() (swrTestValue, error) {
			calls++
			return swrTestValue{Name: "fresh", N: 2}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, swrTestValue{Name: "cached", N: 1}, got)
	require.Equal(t, 0, calls)
}

func TestFetchWithSWRCache_CircuitOpenServesStale(t *testing.T) {
	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	ctx := context.Background()

	key := "test:key"
	staleKey := key + ":stale"
	require.NoError(t, store.Set(ctx, staleKey, swrTestValue{Name: "stale", N: 9}, time.Minute))

	cb := resilience.NewCircuitBreaker(1, time.Hour)
	cb.RecordFailure()
	require.True(t, cb.IsOpen())

	calls := 0
	got, err := FetchWithSWRCache(ctx, SWRCacheOptions[swrTestValue]{
		Store:          store,
		Key:            key,
		FreshTTL:       time.Minute,
		StaleTTL:       time.Minute,
		CircuitBreaker: cb,
		Fetch: func() (swrTestValue, error) {
			calls++
			return swrTestValue{Name: "fresh", N: 1}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, swrTestValue{Name: "stale", N: 9}, got)
	require.Equal(t, 0, calls)
}

func TestFetchWithSWRCache_FetchErrorServesStaleAndRecordsFailure(t *testing.T) {
	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	ctx := context.Background()

	key := "test:key"
	staleKey := key + ":stale"
	require.NoError(t, store.Set(ctx, staleKey, swrTestValue{Name: "stale", N: 3}, time.Minute))

	cb := resilience.NewCircuitBreaker(1, time.Hour)

	calls := 0
	got, err := FetchWithSWRCache(ctx, SWRCacheOptions[swrTestValue]{
		Store:          store,
		Key:            key,
		FreshTTL:       time.Minute,
		StaleTTL:       time.Minute,
		CircuitBreaker: cb,
		Fetch: func() (swrTestValue, error) {
			calls++
			return swrTestValue{}, errors.New("boom")
		},
	})
	require.NoError(t, err)
	require.Equal(t, swrTestValue{Name: "stale", N: 3}, got)
	require.GreaterOrEqual(t, calls, 1)
	require.True(t, cb.IsOpen())
}

func TestFetchWithSWRCache_FetchSuccessCachesFreshAndStale(t *testing.T) {
	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	ctx := context.Background()

	key := "test:key"
	cb := resilience.NewCircuitBreaker(2, time.Hour)
	cb.RecordFailure()
	require.False(t, cb.IsOpen())

	calls := 0
	got, err := FetchWithSWRCache(ctx, SWRCacheOptions[swrTestValue]{
		Store:          store,
		Key:            key,
		FreshTTL:       time.Minute,
		StaleTTL:       time.Minute,
		CircuitBreaker: cb,
		Fetch: func() (swrTestValue, error) {
			calls++
			return swrTestValue{Name: "fresh", N: 7}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, swrTestValue{Name: "fresh", N: 7}, got)
	require.Equal(t, 1, calls)
	require.False(t, cb.IsOpen())

	var cached swrTestValue
	require.NoError(t, store.Get(ctx, key, &cached))
	require.Equal(t, swrTestValue{Name: "fresh", N: 7}, cached)

	var stale swrTestValue
	require.NoError(t, store.Get(ctx, key+":stale", &stale))
	require.Equal(t, swrTestValue{Name: "fresh", N: 7}, stale)
}

func TestFetchWithSWRCache_SingleflightDedupesFetch(t *testing.T) {
	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	ctx := context.Background()

	var sf singleflight.Group
	var called atomic.Int32

	fetch := func() (swrTestValue, error) {
		called.Add(1)
		time.Sleep(25 * time.Millisecond)
		return swrTestValue{Name: "ok", N: 1}, nil
	}

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	errCh := make(chan error, n)
	for range n {
		go func() {
			defer wg.Done()
			got, err := FetchWithSWRCache(ctx, SWRCacheOptions[swrTestValue]{
				Store:           store,
				Key:             "test:key:sf",
				FreshTTL:        time.Minute,
				StaleTTL:        time.Minute,
				Singleflight:    &sf,
				SingleflightKey: "test:key:sf",
				Fetch:           fetch,
			})
			if err != nil {
				errCh <- err
				return
			}
			if got.Name != "ok" || got.N != 1 {
				errCh <- errors.New("unexpected result")
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	require.Equal(t, int32(1), called.Load())
}
