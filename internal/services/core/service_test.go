// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/services/cache"
)

type partialCanceledReadCloser struct {
	read bool
}

func (r *partialCanceledReadCloser) Read(p []byte) (int, error) {
	if r.read {
		return 0, context.Canceled
	}
	r.read = true
	payload := []byte(`{"devices":[`)
	copy(p, payload)
	return len(payload), context.Canceled
}

func (r *partialCanceledReadCloser) Close() error {
	return nil
}

func TestCacheUpdateStatusRoundTrip(t *testing.T) {
	t.Parallel()

	s := &ServiceCore{
		cache: cache.NewMemoryStore(context.Background(), t.TempDir()),
	}

	baseURL := "http://example.com"
	got, found := s.GetUpdateStatusFromCacheWithFound(context.Background(), baseURL)
	if found {
		t.Fatalf("expected cache miss to report found=false")
	}
	if got {
		t.Fatalf("expected cache miss to be false, got true")
	}

	if err := s.CacheUpdateStatus(context.Background(), baseURL, true, time.Minute); err != nil {
		t.Fatalf("CacheUpdateStatus(true) failed: %v", err)
	}
	got, found = s.GetUpdateStatusFromCacheWithFound(context.Background(), baseURL)
	if !found {
		t.Fatalf("expected cached value to report found=true")
	}
	if !got {
		t.Fatalf("expected cached value true, got false")
	}

	if err := s.CacheUpdateStatus(context.Background(), baseURL, false, time.Minute); err != nil {
		t.Fatalf("CacheUpdateStatus(false) failed: %v", err)
	}
	got, found = s.GetUpdateStatusFromCacheWithFound(context.Background(), baseURL)
	if !found {
		t.Fatalf("expected cached value to report found=true")
	}
	if got {
		t.Fatalf("expected cached value false, got true")
	}
}

func TestDoRequest_DelayedBodyReadableAfterReturn(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = w.Write([]byte("hello "))
			flusher.Flush()
		}
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("world"))
	}))
	defer srv.Close()

	s := &ServiceCore{}
	s.SetTimeout(time.Second)

	resp, err := s.DoRequest(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "hello world" {
		t.Fatalf("unexpected response body: %q", string(body))
	}
}

func TestReadBody_PartialCanceledReadReturnsContextCanceled(t *testing.T) {
	t.Parallel()

	s := &ServiceCore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       &partialCanceledReadCloser{},
	}

	body, err := s.ReadBody(resp)
	if err == nil {
		t.Fatalf("expected read error")
	}
	if !errors.Is(err, ErrContextCanceled) {
		t.Fatalf("expected ErrContextCanceled, got: %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil body, got %q", string(body))
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

	got, found := s.GetUpdateStatusFromCacheWithFound(context.Background(), baseURL)
	if !found {
		t.Fatalf("expected legacy cached value to report found=true")
	}
	if !got {
		t.Fatalf("expected legacy cached value true, got false")
	}
}
