// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package whisparr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/types"
)

// Whisparr V2 is a Sonarr V3 fork, so the service must speak /api/v3 and
// authenticate with X-Api-Key. A wrong default port or an API version copied
// from Lidarr would break these without failing to compile.
func TestNewWhisparrServiceDefaults(t *testing.T) {
	t.Parallel()

	service, ok := NewWhisparrService().(*WhisparrService)
	if !ok {
		t.Fatalf("NewWhisparrService did not return *WhisparrService")
	}

	if got, want := service.Type, "whisparr"; got != want {
		t.Errorf("Type = %q, want %q", got, want)
	}
	if got, want := service.DisplayName, "Whisparr"; got != want {
		t.Errorf("DisplayName = %q, want %q", got, want)
	}
	if got, want := service.DefaultURL, "http://localhost:6969"; got != want {
		t.Errorf("DefaultURL = %q, want %q", got, want)
	}
	if got, want := service.HealthEndpoint, "/api/v3/health"; got != want {
		t.Errorf("HealthEndpoint = %q, want %q", got, want)
	}
}

func TestGetHealthEndpoint(t *testing.T) {
	t.Parallel()

	service := &WhisparrService{}

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"plain", "http://localhost:6969", "http://localhost:6969/api/v3/health"},
		{"trailing slash", "http://localhost:6969/", "http://localhost:6969/api/v3/health"},
		{"repeated trailing slashes", "http://localhost:6969///", "http://localhost:6969/api/v3/health"},
		{"subpath", "https://example.com/whisparr", "https://example.com/whisparr/api/v3/health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := service.GetHealthEndpoint(tt.baseURL); got != tt.want {
				t.Errorf("GetHealthEndpoint(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestDeleteQueueItem_ValidationErrorsAreArrErrors(t *testing.T) {
	t.Parallel()

	service := &WhisparrService{}

	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantMsg string
	}{
		{"missing url", "", "key", "URL is required"},
		{"missing api key", "http://localhost:6969", "", "API key is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := service.DeleteQueueItem(
				context.Background(),
				tt.baseURL,
				tt.apiKey,
				"123",
				types.WhisparrQueueDeleteOptions{},
			)

			var arrErr *arr.ErrArr
			if !errors.As(err, &arrErr) {
				t.Fatalf("expected *arr.ErrArr, got %T (%v)", err, err)
			}
			if arrErr.Service != "whisparr" || arrErr.Op != "delete_queue" {
				t.Fatalf("unexpected arr error fields: %+v", arrErr)
			}
			if arrErr.Err == nil || arrErr.Err.Error() != tt.wantMsg {
				t.Fatalf("unexpected validation message: got %v want %q", arrErr.Err, tt.wantMsg)
			}
		})
	}
}

func TestGetQueueForHealth_RequestShapeAndParsing(t *testing.T) {
	t.Parallel()

	var gotPath, gotAPIKey, gotQuery, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAPIKey = r.URL.Path, r.Header.Get("X-Api-Key")
		gotQuery, gotMethod = r.URL.RawQuery, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"totalRecords":2,"records":[
			{"id":11,"title":"Scene A","status":"downloading","size":1024,"sizeleft":512,
			 "protocol":"torrent","indexer":"idx","downloadClient":"qbit",
			 "trackedDownloadState":"importBlocked","customFormatScore":7},
			{"id":12,"title":"Scene B","status":"completed","size":2048}]}`))
	}))
	defer srv.Close()

	records, err := (&WhisparrService{}).GetQueueForHealth(context.Background(), srv.URL, "secret-key")
	if err != nil {
		t.Fatalf("GetQueueForHealth returned error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/queue" {
		t.Errorf("path = %q, want /api/v3/queue", gotPath)
	}
	if gotAPIKey != "secret-key" {
		t.Errorf("X-Api-Key = %q, want secret-key", gotAPIKey)
	}
	// Sonarr-fork queue params: series/episode, not movie or artist.
	for _, want := range []string{
		"includeUnknownSeriesItems=false",
		"includeSeries=true",
		"includeEpisode=true",
	} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	first := records[0]
	if first.ID != 11 || first.Title != "Scene A" || first.Status != "downloading" {
		t.Errorf("unexpected first record: %+v", first)
	}
	if first.Size != 1024 || first.SizeLeft != 512 {
		t.Errorf("size fields not parsed: size=%d sizeleft=%d", first.Size, first.SizeLeft)
	}
	if first.TrackedDownloadState != "importBlocked" {
		t.Errorf("trackedDownloadState = %q, want importBlocked", first.TrackedDownloadState)
	}
	if first.CustomFormatScore != 7 {
		t.Errorf("customFormatScore = %d, want 7", first.CustomFormatScore)
	}
}

func TestGetQueueForHealth_EmptyRecordsIsNotNil(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"totalRecords":0,"records":[]}`))
	}))
	defer srv.Close()

	records, err := (&WhisparrService{}).GetQueueForHealth(context.Background(), srv.URL, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records == nil {
		t.Fatal("records is nil; handler and poller rely on a non-nil empty slice")
	}
	if len(records) != 0 {
		t.Fatalf("got %d records, want 0", len(records))
	}
}

// A rejected API key is the most common misconfiguration; it must surface as an
// *arr.ErrArr carrying the HTTP status rather than as a parse failure.
func TestGetQueueForHealth_UnauthorizedSurfacesStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := (&WhisparrService{}).GetQueueForHealth(context.Background(), srv.URL, "wrong-key")

	var arrErr *arr.ErrArr
	if !errors.As(err, &arrErr) {
		t.Fatalf("expected *arr.ErrArr, got %T (%v)", err, err)
	}
	if arrErr.Service != "whisparr" || arrErr.Op != "get_queue" {
		t.Fatalf("unexpected arr error fields: %+v", arrErr)
	}
	if arrErr.HttpCode != http.StatusUnauthorized {
		t.Fatalf("HttpCode = %d, want %d", arrErr.HttpCode, http.StatusUnauthorized)
	}
}

func TestDeleteQueueItem_SendsV3DeleteWithOptions(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.Query()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&WhisparrService{}).DeleteQueueItem(
		context.Background(), srv.URL, "key", "42",
		types.WhisparrQueueDeleteOptions{RemoveFromClient: true, Blocklist: true, ChangeCategory: true},
	)
	if err != nil {
		t.Fatalf("DeleteQueueItem returned error: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v3/queue/42" {
		t.Errorf("path = %q, want /api/v3/queue/42", gotPath)
	}
	for key, want := range map[string]string{
		"removeFromClient": "true",
		"blocklist":        "true",
		"skipRedownload":   "false",
		"changeCategory":   "true",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}
}
