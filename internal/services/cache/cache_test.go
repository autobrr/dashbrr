// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

func setupTestCache(t *testing.T) Store {
	t.Helper()

	store, err := InitCache(context.Background(), Config{
		DataDir: t.TempDir(),
		testing: true, // bypass singleton for isolated integration tests
	})
	require.NoError(t, err)
	return store
}

func TestInitCache(t *testing.T) {
	store, err := InitCache(context.Background(), Config{
		DataDir: t.TempDir(),
		testing: true,
	})
	require.NoError(t, err)
	require.NotNil(t, store)
	_, ok := store.(*MemoryStore)
	assert.True(t, ok, "expected MemoryStore type")
	require.NoError(t, store.Close())
}

func TestBasicOperations(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	tests := []struct {
		name      string
		key       string
		value     testStruct
		ttl       time.Duration
		wantError bool
	}{
		{
			name:      "Basic set and get",
			key:       "test:basic",
			value:     testStruct{Name: "test", Value: 123},
			ttl:       time.Minute,
			wantError: false,
		},
		{
			name:      "Zero TTL",
			key:       "test:zero_ttl",
			value:     testStruct{Name: "zero", Value: 456},
			ttl:       0,
			wantError: false,
		},
		{
			name:      "Health prefix",
			key:       "health:test",
			value:     testStruct{Name: "health", Value: 789},
			ttl:       0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			var retrieved testStruct
			err = cache.Get(ctx, tt.key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, tt.value, retrieved)

			err = cache.Delete(ctx, tt.key)
			require.NoError(t, err)

			err = cache.Get(ctx, tt.key, &retrieved)
			assert.Equal(t, ErrKeyNotFound, err)
		})
	}
}

func TestRateLimitOperations(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	key := "test:rate:limit"
	now := time.Now().Unix()

	timestamps := []int64{
		now,     // current (keep)
		now - 1, // 1 second ago (keep)
		now - 3, // 3 seconds ago (remove)
	}

	for i := len(timestamps) - 1; i >= 0; i-- {
		err := cache.Increment(ctx, key, timestamps[i])
		require.NoError(t, err, "Failed to increment")
	}

	count, err := cache.GetCount(ctx, key)
	require.NoError(t, err, "Failed to get initial count")
	assert.Equal(t, int64(3), count, "Expected initial count to be 3")

	err = cache.CleanAndCount(ctx, key, now-2)
	require.NoError(t, err, "Failed to clean and count")

	count, err = cache.GetCount(ctx, key)
	require.NoError(t, err, "Failed to get count after cleanup")
	assert.Equal(t, int64(2), count, "Expected count to be 2 after cleanup")

	err = cache.Expire(ctx, key, time.Second)
	require.NoError(t, err, "Failed to set expiration")

	time.Sleep(2 * time.Second)

	count, err = cache.GetCount(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestConcurrentAccess(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	key := "test:concurrent"
	value := testStruct{Name: "concurrent", Value: 123}
	const numGoroutines = 10
	done := make(chan bool)

	err := cache.Set(ctx, key, value, time.Minute)
	require.NoError(t, err)

	for i := range numGoroutines {
		go func(i int) {
			var retrieved testStruct
			err := cache.Get(ctx, key, &retrieved)
			assert.NoError(t, err)

			newValue := testStruct{Name: "concurrent", Value: i}
			err = cache.Set(ctx, key, newValue, time.Minute)
			assert.NoError(t, err)

			done <- true
		}(i)
	}

	for range numGoroutines {
		<-done
	}
}
