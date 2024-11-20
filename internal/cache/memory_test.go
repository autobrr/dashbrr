// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	store := NewMemoryStore(context.Background(), tempDir)
	defer store.Close()

	ctx := context.Background()

	t.Run("Basic Operations", func(t *testing.T) {
		// Test Set and Get
		key := "test_key"
		value := "test_value"
		err := store.Set(ctx, key, value, time.Minute)
		if err != nil {
			t.Errorf("Failed to set value: %v", err)
		}

		var result string
		err = store.Get(ctx, key, &result)
		if err != nil {
			t.Errorf("Failed to get value: %v", err)
		}
		if result != value {
			t.Errorf("Expected %v, got %v", value, result)
		}

		// Test Delete
		err = store.Delete(ctx, key)
		if err != nil {
			t.Errorf("Failed to delete value: %v", err)
		}

		err = store.Get(ctx, key, &result)
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		key := "expiring_key"
		value := "expiring_value"
		err := store.Set(ctx, key, value, 50*time.Millisecond)
		if err != nil {
			t.Errorf("Failed to set value: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		var result string
		err = store.Get(ctx, key, &result)
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound for expired key, got %v", err)
		}
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		key := "rate_key"
		now := time.Now().Unix()

		// Add some timestamps
		for i := int64(0); i < 5; i++ {
			err := store.Increment(ctx, key, now+i)
			if err != nil {
				t.Errorf("Failed to increment: %v", err)
			}
		}

		// Check count
		count, err := store.GetCount(ctx, key)
		if err != nil {
			t.Errorf("Failed to get count: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected count 5, got %d", count)
		}

		// Clean old entries
		err = store.CleanAndCount(ctx, key, now+3)
		if err != nil {
			t.Errorf("Failed to clean and count: %v", err)
		}

		// Check count after cleaning
		count, err = store.GetCount(ctx, key)
		if err != nil {
			t.Errorf("Failed to get count after cleaning: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected count 2 after cleaning, got %d", count)
		}
	})

	t.Run("Expire", func(t *testing.T) {
		key := "expire_key"
		value := "expire_value"
		err := store.Set(ctx, key, value, time.Minute)
		if err != nil {
			t.Errorf("Failed to set value: %v", err)
		}

		// Update expiration
		err = store.Expire(ctx, key, 50*time.Millisecond)
		if err != nil {
			t.Errorf("Failed to update expiration: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		var result string
		err = store.Get(ctx, key, &result)
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound for expired key, got %v", err)
		}
	})

	t.Run("Concurrent Operations", func(t *testing.T) {
		key := "concurrent_key"
		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				store.Set(ctx, key, i, time.Minute)
			}
			done <- true
		}()

		go func() {
			var result int
			for i := 0; i < 100; i++ {
				store.Get(ctx, key, &result)
			}
			done <- true
		}()

		<-done
		<-done
	})
}

func TestMemoryStoreClose(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	store := NewMemoryStore(context.Background(), tempDir)

	// Test normal operations
	ctx := context.Background()
	err := store.Set(ctx, "key", "value", time.Minute)
	if err != nil {
		t.Errorf("Failed to set value before close: %v", err)
	}

	// Close the store
	err = store.Close()
	if err != nil {
		t.Errorf("Failed to close store: %v", err)
	}

	// Verify operations fail after close
	err = store.Set(ctx, "key2", "value2", time.Minute)
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed after close, got %v", err)
	}

	var result string
	err = store.Get(ctx, "key", &result)
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed after close, got %v", err)
	}
}

func TestMemoryStorePersistence(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a store and add some data
	store := NewMemoryStore(context.Background(), tempDir)
	ctx := context.Background()

	err := store.Set(ctx, "session:test", "test_value", time.Hour)
	if err != nil {
		t.Errorf("Failed to set value: %v", err)
	}

	// Close the store
	err = store.Close()
	if err != nil {
		t.Errorf("Failed to close store: %v", err)
	}

	// Create a new store with the same directory
	store2 := NewMemoryStore(context.Background(), tempDir)
	defer store2.Close()

	// Try to get the persisted value
	var result string
	err = store2.Get(ctx, "session:test", &result)
	if err != nil {
		t.Errorf("Failed to get persisted value: %v", err)
	}
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", result)
	}
}
