// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build integration

package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

func setupTestDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "dashbrr-test-"+time.Now().Format("20060102150405"))
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err, "Failed to create test directory")
	return dir
}

// checkRedisAvailable checks if Redis is available at the given address
func checkRedisAvailable(addr string) bool {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	return err == nil
}

func setupTestCache(t *testing.T) Store {
	dataDir := setupTestDir(t)
	cfg := Config{
		RedisAddr: "localhost:6379",
		DataDir:   dataDir,
		testing:   true, // Enable test mode to bypass singleton pattern
	}

	// Check if Redis is available
	if !checkRedisAvailable(cfg.RedisAddr) {
		t.Skip("Redis not available, skipping test")
	}

	store, err := InitCache(context.Background(), cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.RemoveAll(dataDir); err != nil {
			t.Errorf("Failed to cleanup test directory: %v", err)
		}
	})

	return store
}

func TestInitCache(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantRedis bool // true if we expect a RedisStore, false for MemoryStore
	}{
		{
			name: "Valid Redis address",
			config: Config{
				RedisAddr: "localhost:6379",
				DataDir:   setupTestDir(t),
				testing:   true,
			},
			wantRedis: true,
		},
		{
			name: "Invalid Redis address",
			config: Config{
				RedisAddr: "invalid:6379",
				DataDir:   setupTestDir(t),
				testing:   true,
			},
			wantRedis: false,
		},
		{
			name: "No Redis configured",
			config: Config{
				DataDir: setupTestDir(t),
				testing: true,
			},
			wantRedis: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantRedis && !checkRedisAvailable(tt.config.RedisAddr) {
				t.Skip("Redis not available, skipping test")
			}

			store, err := InitCache(context.Background(), tt.config)
			require.NotNil(t, store, "Store should never be nil")

			if tt.wantRedis {
				redisStore, ok := store.(*RedisStore)
				assert.True(t, ok, "Expected RedisStore type")
				if ok {
					err = redisStore.Close()
					assert.NoError(t, err)
				}
			} else {
				_, ok := store.(*MemoryStore)
				assert.True(t, ok, "Expected MemoryStore type")
				err = store.Close()
				assert.NoError(t, err)
			}

			// Clean up test directory
			err = os.RemoveAll(tt.config.DataDir)
			assert.NoError(t, err)
		})
	}
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
			// Test Set
			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Test Get
			var retrieved testStruct
			err = cache.Get(ctx, tt.key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, tt.value, retrieved)

			// Test Delete
			err = cache.Delete(ctx, tt.key)
			require.NoError(t, err)

			// Verify deletion
			err = cache.Get(ctx, tt.key, &retrieved)
			assert.Equal(t, ErrKeyNotFound, err)
		})
	}
}

func TestLocalCache(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	tests := []struct {
		name  string
		key   string
		value testStruct
		ttl   time.Duration
	}{
		{
			name:  "Short TTL",
			key:   "test:local_short",
			value: testStruct{Name: "local", Value: 456},
			ttl:   time.Second,
		},
		{
			name:  "Regular TTL",
			key:   "test:local_regular",
			value: testStruct{Name: "local2", Value: 789},
			ttl:   time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set value
			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)
			require.NoError(t, err)

			// Get value should use local cache
			var retrieved testStruct
			err = cache.Get(ctx, tt.key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, tt.value, retrieved)

			if tt.ttl == time.Second {
				// Wait for short TTL to expire
				time.Sleep(time.Second * 2)
				err = cache.Get(ctx, tt.key, &retrieved)
				assert.Equal(t, ErrKeyNotFound, err, "Cache entry should have expired")
			}
		})
	}
}

func TestRateLimitOperations(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	key := "test:rate:limit"
	now := time.Now().Unix()

	// Add entries with specific timestamps
	timestamps := []int64{
		now,     // current (keep)
		now - 1, // 1 second ago (keep)
		now - 3, // 3 seconds ago (remove)
	}

	// Add entries in reverse order to ensure sorting works
	for i := len(timestamps) - 1; i >= 0; i-- {
		err := cache.Increment(ctx, key, timestamps[i])
		require.NoError(t, err, "Failed to increment")
	}

	// Verify initial count
	count, err := cache.GetCount(ctx, key)
	require.NoError(t, err, "Failed to get initial count")
	assert.Equal(t, int64(3), count, "Expected initial count to be 3")

	// Clean entries older than now-2
	// This should remove entries with timestamp <= now-2
	err = cache.CleanAndCount(ctx, key, now-2)
	require.NoError(t, err, "Failed to clean and count")

	// Verify count after cleanup
	count, err = cache.GetCount(ctx, key)
	require.NoError(t, err, "Failed to get count after cleanup")
	assert.Equal(t, int64(2), count, "Expected count to be 2 after cleanup")

	// Test expiration
	err = cache.Expire(ctx, key, time.Second)
	require.NoError(t, err, "Failed to set expiration")

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Verify expiration
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

	// Set initial value
	err := cache.Set(ctx, key, value, time.Minute)
	require.NoError(t, err)

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			// Mix of operations
			var retrieved testStruct
			err := cache.Get(ctx, key, &retrieved)
			assert.NoError(t, err)

			newValue := testStruct{Name: "concurrent", Value: i}
			err = cache.Set(ctx, key, newValue, time.Minute)
			assert.NoError(t, err)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestCleanup(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	ctx := context.Background()
	key := "test:cleanup"
	value := testStruct{Name: "cleanup", Value: 123}

	// Set with short TTL
	err := cache.Set(ctx, key, value, time.Second)
	require.NoError(t, err)

	// Verify initial set
	var retrieved testStruct
	err = cache.Get(ctx, key, &retrieved)
	require.NoError(t, err)

	// Wait for cleanup
	time.Sleep(time.Second * 2)

	// Verify cleanup
	err = cache.Get(ctx, key, &retrieved)
	assert.Equal(t, ErrKeyNotFound, err, "Cache entry should have been cleaned up")
}

func TestContextCancellation(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	key := "test:context"
	value := testStruct{Name: "context", Value: 123}

	// Attempt operations with cancelled context
	err := cache.Set(ctx, key, value, time.Minute)
	assert.Error(t, err, "Expected error with cancelled context")
	assert.Equal(t, context.Canceled, err)

	var retrieved testStruct
	err = cache.Get(ctx, key, &retrieved)
	assert.Error(t, err, "Expected error with cancelled context")
	assert.Equal(t, context.Canceled, err)

	// Test timeout context
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // Ensure timeout occurs

	err = cache.Set(timeoutCtx, key, value, time.Minute)
	assert.Error(t, err, "Expected error with timeout context")
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestClosedCache(t *testing.T) {
	cache := setupTestCache(t)

	// Close the cache
	err := cache.Close()
	require.NoError(t, err)

	ctx := context.Background()
	key := "test:closed"
	value := testStruct{Name: "closed", Value: 123}

	// Attempt operations on closed cache
	err = cache.Set(ctx, key, value, time.Minute)
	assert.Equal(t, ErrClosed, err)

	var retrieved testStruct
	err = cache.Get(ctx, key, &retrieved)
	assert.Equal(t, ErrClosed, err)

	err = cache.Delete(ctx, key)
	assert.Equal(t, ErrClosed, err)
}
