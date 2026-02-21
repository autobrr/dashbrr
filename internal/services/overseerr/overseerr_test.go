// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package overseerr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/autobrr/dashbrr/internal/types"
)

func TestGetRequests_DoesNotPerformPerRequestLookups(t *testing.T) {
	clearTitleCache()

	var requestCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request":
			atomic.AddInt32(&requestCalls, 1)
			w.Header().Set("Content-Type", "application/json")

			response := types.RequestsResponse{
				Results: []types.MediaRequest{
					{
						ID:     1,
						Status: 1,
						Media: struct {
							ID                int      `json:"id"`
							TmdbID            int      `json:"tmdbId"`
							TvdbID            int      `json:"tvdbId"`
							Status            int      `json:"status"`
							Requests          []string `json:"requests"`
							CreatedAt         string   `json:"createdAt"`
							UpdatedAt         string   `json:"updatedAt"`
							MediaType         string   `json:"mediaType"`
							ServiceUrl        string   `json:"serviceUrl"`
							Title             string   `json:"title,omitempty"`
							ExternalServiceID int      `json:"externalServiceId,omitempty"`
						}{
							MediaType: "movie",
							Title:     "Movie A",
						},
					},
					{
						ID:     2,
						Status: 2,
						Media: struct {
							ID                int      `json:"id"`
							TmdbID            int      `json:"tmdbId"`
							TvdbID            int      `json:"tvdbId"`
							Status            int      `json:"status"`
							Requests          []string `json:"requests"`
							CreatedAt         string   `json:"createdAt"`
							UpdatedAt         string   `json:"updatedAt"`
							MediaType         string   `json:"mediaType"`
							ServiceUrl        string   `json:"serviceUrl"`
							Title             string   `json:"title,omitempty"`
							ExternalServiceID int      `json:"externalServiceId,omitempty"`
						}{
							MediaType: "tv",
							Title:     "Show B",
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &OverseerrService{}
	stats, err := svc.GetRequests(context.Background(), server.URL, "key")
	if err != nil {
		t.Fatalf("GetRequests() error = %v", err)
	}
	if stats == nil {
		t.Fatal("GetRequests() returned nil stats")
	}
	if stats.PendingCount != 1 {
		t.Fatalf("PendingCount = %d, want 1", stats.PendingCount)
	}
	if len(stats.Requests) != 2 {
		t.Fatalf("requests len = %d, want 2", len(stats.Requests))
	}
	if stats.Requests[0].Media.Title != "Movie A" || stats.Requests[1].Media.Title != "Show B" {
		t.Fatalf("expected titles preserved from Overseerr payload, got %+v", stats.Requests)
	}
	if got := atomic.LoadInt32(&requestCalls); got != 1 {
		t.Fatalf("request endpoint calls = %d, want 1", got)
	}
}

func TestGetRequests_EnrichesMissingTitlesFromOverseerr(t *testing.T) {
	clearTitleCache()

	var requestCalls int32
	var movieCalls int32
	var tvCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/request":
			atomic.AddInt32(&requestCalls, 1)
			response := types.RequestsResponse{
				Results: []types.MediaRequest{
					{
						ID:     1,
						Status: 1,
						Media: struct {
							ID                int      `json:"id"`
							TmdbID            int      `json:"tmdbId"`
							TvdbID            int      `json:"tvdbId"`
							Status            int      `json:"status"`
							Requests          []string `json:"requests"`
							CreatedAt         string   `json:"createdAt"`
							UpdatedAt         string   `json:"updatedAt"`
							MediaType         string   `json:"mediaType"`
							ServiceUrl        string   `json:"serviceUrl"`
							Title             string   `json:"title,omitempty"`
							ExternalServiceID int      `json:"externalServiceId,omitempty"`
						}{
							MediaType: "movie",
							TmdbID:    780609,
						},
					},
					{
						ID:     2,
						Status: 2,
						Media: struct {
							ID                int      `json:"id"`
							TmdbID            int      `json:"tmdbId"`
							TvdbID            int      `json:"tvdbId"`
							Status            int      `json:"status"`
							Requests          []string `json:"requests"`
							CreatedAt         string   `json:"createdAt"`
							UpdatedAt         string   `json:"updatedAt"`
							MediaType         string   `json:"mediaType"`
							ServiceUrl        string   `json:"serviceUrl"`
							Title             string   `json:"title,omitempty"`
							ExternalServiceID int      `json:"externalServiceId,omitempty"`
						}{
							MediaType: "tv",
							TmdbID:    405593,
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case "/api/v1/movie/780609":
			atomic.AddInt32(&movieCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]string{"title": "Movie Title"})
		case "/api/v1/tv/405593":
			atomic.AddInt32(&tvCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "Show Title"})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &OverseerrService{}
	stats, err := svc.GetRequests(context.Background(), server.URL, "key")
	if err != nil {
		t.Fatalf("GetRequests() error = %v", err)
	}
	if stats == nil {
		t.Fatal("GetRequests() returned nil stats")
	}
	if len(stats.Requests) != 2 {
		t.Fatalf("requests len = %d, want 2", len(stats.Requests))
	}
	if stats.Requests[0].Media.Title != "Movie Title" {
		t.Fatalf("movie title = %q, want %q", stats.Requests[0].Media.Title, "Movie Title")
	}
	if stats.Requests[1].Media.Title != "Show Title" {
		t.Fatalf("show title = %q, want %q", stats.Requests[1].Media.Title, "Show Title")
	}
	if atomic.LoadInt32(&requestCalls) != 1 {
		t.Fatalf("request endpoint calls = %d, want 1", requestCalls)
	}
	if atomic.LoadInt32(&movieCalls) != 1 {
		t.Fatalf("movie lookup calls = %d, want 1", movieCalls)
	}
	if atomic.LoadInt32(&tvCalls) != 1 {
		t.Fatalf("tv lookup calls = %d, want 1", tvCalls)
	}
}

func TestGetRequests_UsesTitleCacheForRepeatedLookups(t *testing.T) {
	clearTitleCache()

	var requestCalls int32
	var movieCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/request":
			atomic.AddInt32(&requestCalls, 1)
			response := types.RequestsResponse{
				Results: []types.MediaRequest{
					{
						ID:     1,
						Status: 1,
						Media: struct {
							ID                int      `json:"id"`
							TmdbID            int      `json:"tmdbId"`
							TvdbID            int      `json:"tvdbId"`
							Status            int      `json:"status"`
							Requests          []string `json:"requests"`
							CreatedAt         string   `json:"createdAt"`
							UpdatedAt         string   `json:"updatedAt"`
							MediaType         string   `json:"mediaType"`
							ServiceUrl        string   `json:"serviceUrl"`
							Title             string   `json:"title,omitempty"`
							ExternalServiceID int      `json:"externalServiceId,omitempty"`
						}{
							MediaType: "movie",
							TmdbID:    780609,
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case "/api/v1/movie/780609":
			atomic.AddInt32(&movieCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]string{"title": "Movie Title"})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &OverseerrService{}
	for range 2 {
		stats, err := svc.GetRequests(context.Background(), server.URL, "key")
		if err != nil {
			t.Fatalf("GetRequests() error = %v", err)
		}
		if len(stats.Requests) != 1 {
			t.Fatalf("requests len = %d, want 1", len(stats.Requests))
		}
		if stats.Requests[0].Media.Title != "Movie Title" {
			t.Fatalf("movie title = %q, want %q", stats.Requests[0].Media.Title, "Movie Title")
		}
	}

	if atomic.LoadInt32(&requestCalls) != 2 {
		t.Fatalf("request endpoint calls = %d, want 2", requestCalls)
	}
	if atomic.LoadInt32(&movieCalls) != 1 {
		t.Fatalf("movie lookup calls = %d, want 1", movieCalls)
	}
}

func clearTitleCache() {
	titleCacheMu.Lock()
	titleCache = make(map[string]titleCacheEntry)
	titleCacheMu.Unlock()
}
