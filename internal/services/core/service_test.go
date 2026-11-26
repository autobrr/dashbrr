// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "token in query", raw: "https://seedpool.org/api/user?api_token=secret", want: "https://seedpool.org/api/user"},
		{name: "userinfo", raw: "http://user:pass@sabnzbd:8080/api", want: "http://sabnzbd:8080/api"}, //nolint:gosec // test fixture
		{name: "token in fragment", raw: "https://example.test/api#access_token=secret", want: "https://example.test/api"},
		{name: "plain", raw: "http://traefik:8080/metrics", want: "http://traefik:8080/metrics"},
		{name: "unparseable", raw: "http://host:port?token=secret", want: "[unparseable url]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactURL(tt.raw); got != tt.want {
				t.Fatalf("RedactURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDoRequest_ErrorOmitsQuerySecret(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	s := &ServiceCore{}
	s.SetTimeout(time.Second)

	_, err := s.DoRequest(t.Context(), http.MethodGet, srv.URL+"/api?api_token=secret", nil, nil)
	if err == nil {
		t.Fatal("DoRequest succeeded against a closed server")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error contains the query secret: %v", err)
	}
}

func TestReadBody_StatusErrors(t *testing.T) {
	t.Parallel()

	const rawBody = `{"error":"Unauthorized"}`
	tests := []struct {
		status      int
		contentType string
		wantErr     string
	}{
		{http.StatusUnauthorized, "application/json", "Invalid API key"},
		{http.StatusForbidden, "text/plain", "Access forbidden"},
		{http.StatusNotFound, "text/html", "Service endpoint not found"},
		{http.StatusBadGateway, "text/html", "Service is temporarily unavailable (502 Bad Gateway)"},
		{http.StatusServiceUnavailable, "application/json", "Service is temporarily unavailable (503 Service Unavailable)"},
		{http.StatusGatewayTimeout, "text/html", "Service is temporarily unavailable (504 Gateway Timeout)"},
		{http.StatusInternalServerError, "text/plain", "Server returned Internal Server Error (500)"},
		{http.StatusInternalServerError, "application/json", ""},
		{http.StatusOK, "application/json", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d %s", tt.status, tt.contentType), func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(rawBody))
			}))
			defer srv.Close()

			s := &ServiceCore{}
			resp, err := s.DoRequest(t.Context(), http.MethodGet, srv.URL, nil, nil)
			if err != nil {
				t.Fatalf("DoRequest failed: %v", err)
			}

			body, err := s.ReadBody(resp)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ReadBody error = %v, want nil", err)
				}
				if string(body) != rawBody {
					t.Fatalf("ReadBody body = %q, want %q", body, rawBody)
				}
				return
			}
			statusErr, ok := errors.AsType[*HTTPStatusError](err)
			if !ok {
				t.Fatalf("ReadBody error = %v, want *HTTPStatusError", err)
			}
			if statusErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, tt.status)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("ReadBody error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestReadBody_StatusErrorBodyOnlyInDebugLog(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previous })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("raw upstream detail"))
	}))
	defer srv.Close()

	s := &ServiceCore{Type: "sonarr"}
	resp, err := s.DoRequest(t.Context(), http.MethodGet, srv.URL+"/api?apikey=secret-key", nil, nil)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}
	if _, err := s.ReadBody(resp); err == nil || strings.Contains(err.Error(), "raw upstream detail") {
		t.Fatalf("ReadBody error = %v, want a status error without the body", err)
	}

	type logEntry struct {
		Level  string `json:"level"`
		URL    string `json:"url"`
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	var entries []logEntry
	for line := range strings.Lines(logs.String()) {
		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		entries = append(entries, entry)
	}

	wantURL := srv.URL + "/api"
	want := []logEntry{
		{Level: zerolog.LevelWarnValue, URL: wantURL, Status: http.StatusUnauthorized},
		{Level: zerolog.LevelDebugValue, URL: wantURL, Status: http.StatusUnauthorized, Body: "raw upstream detail"},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("log entries = %+v, want %+v", entries, want)
	}
}
