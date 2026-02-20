// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package jellyfin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSessions_UsesSessionEndpointAndFiltersNowPlaying(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Sessions" {
			t.Fatalf("path = %s, want /Sessions", r.URL.Path)
		}
		if got := r.URL.Query().Get("ActiveWithinSeconds"); got != "300" {
			t.Fatalf("ActiveWithinSeconds = %q, want 300", got)
		}
		if got := r.Header.Get("X-Emby-Token"); got != "abc123" {
			t.Fatalf("X-Emby-Token = %q, want abc123", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"Id":"1","UserName":"alice","NowPlayingItem":{"Name":"Movie A","Type":"Movie","RunTimeTicks":36000000000}},
			{"Id":"2","UserName":"bob"},
			{"Id":"3","UserName":"carol","NowPlayingItem":{"Name":"","SeriesName":"","Type":"Episode"}}
		]`))
	}))
	defer server.Close()

	service := NewJellyfinService().(*JellyfinService)
	sessions, err := service.GetSessions(context.Background(), server.URL, "abc123")
	if err != nil {
		t.Fatalf("GetSessions error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "1" {
		t.Fatalf("sessions[0].ID = %q, want 1", sessions[0].ID)
	}
}

func TestGetSummary_CombinesSystemAndSessions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/System/Info":
			_, _ = w.Write([]byte(`{"ServerName":"Jellyfin","Version":"10.10.7","ProductName":"Jellyfin Server","Id":"srv-1"}`))
		case "/Sessions":
			_, _ = w.Write([]byte(`[
				{"Id":"1","UserName":"alice","NowPlayingItem":{"Name":"Movie A","Type":"Movie","RunTimeTicks":36000000000}}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewJellyfinService().(*JellyfinService)
	summary, err := service.GetSummary(context.Background(), server.URL, "abc123")
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if summary.System.Version != "10.10.7" {
		t.Fatalf("version = %q, want 10.10.7", summary.System.Version)
	}
	if len(summary.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(summary.Sessions))
	}
}

func TestCheckHealth_ReturnsOnlineWithVersion(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"Media","Version":"10.10.7","ProductName":"Jellyfin Server"}`))
	}))
	defer server.Close()

	service := NewJellyfinService().(*JellyfinService)
	health, statusCode := service.CheckHealth(context.Background(), server.URL, "abc123")
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("health.Status = %q, want online", health.Status)
	}
	if health.Version != "10.10.7" {
		t.Fatalf("health.Version = %q, want 10.10.7", health.Version)
	}
	if !strings.Contains(health.Message, "Media") {
		t.Fatalf("message = %q, want server name", health.Message)
	}
}
