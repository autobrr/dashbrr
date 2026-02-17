// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"context"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/services/cache"
)

func TestCacheUpdateStatusRoundTrip(t *testing.T) {
	t.Parallel()

	s := &ServiceCore{
		cache: cache.NewMemoryStore(context.Background(), t.TempDir()),
	}

	baseURL := "http://example.com"
	if got := s.GetUpdateStatusFromCache(context.Background(), baseURL); got {
		t.Fatalf("expected cache miss to be false, got true")
	}

	if err := s.CacheUpdateStatus(context.Background(), baseURL, true, time.Minute); err != nil {
		t.Fatalf("CacheUpdateStatus(true) failed: %v", err)
	}
	if got := s.GetUpdateStatusFromCache(context.Background(), baseURL); !got {
		t.Fatalf("expected cached value true, got false")
	}

	if err := s.CacheUpdateStatus(context.Background(), baseURL, false, time.Minute); err != nil {
		t.Fatalf("CacheUpdateStatus(false) failed: %v", err)
	}
	if got := s.GetUpdateStatusFromCache(context.Background(), baseURL); got {
		t.Fatalf("expected cached value false, got true")
	}
}

func TestGetUpdateStatusFromCache_LegacyVersionPrefixedKey(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	s := &ServiceCore{cache: store}

	baseURL := "http://legacy.example"
	legacyKey := "version:" + baseURL + ":update"

	if err := store.Set(context.Background(), legacyKey, "true", time.Minute); err != nil {
		t.Fatalf("failed to seed legacy key: %v", err)
	}

	if got := s.GetUpdateStatusFromCache(context.Background(), baseURL); !got {
		t.Fatalf("expected legacy cached value true, got false")
	}
}
