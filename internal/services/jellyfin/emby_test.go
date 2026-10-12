// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package jellyfin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeEmby serves the Emby endpoints only under /emby/ and only with the
// X-Emby-Token header. It fails the test on any other request.
func newFakeEmby(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("%s: Authorization header = %q, want none", r.URL.Path, r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Emby-Token") != "abc123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/emby/System/Info":
			_, _ = w.Write([]byte(`{"ServerName":"Emby","Version":"4.10.1.0","Id":"srv-1"}`))
		case "/emby/Sessions":
			if r.URL.Query().Has("ActiveWithinSeconds") {
				t.Errorf("Sessions query = %q, want no ActiveWithinSeconds", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[
				{"Id":"1","UserName":"alice","PlayState":{"PositionTicks":100,"IsPaused":false},"NowPlayingItem":{"Name":"Movie A","Type":"Movie"}},
				{"Id":"2","UserName":"bob"}
			]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestEmbyCheckHealth_ReturnsOnlineWithVersion(t *testing.T) {
	t.Parallel()

	server := newFakeEmby(t)

	for _, suffix := range []string{"", "/", "/emby", "/emby/"} {
		t.Run("url"+suffix, func(t *testing.T) {
			health, status := NewEmbyService().CheckHealth(t.Context(), server.URL+suffix, "abc123")
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if health.Status != "online" {
				t.Fatalf("health status = %q, want online (message %q)", health.Status, health.Message)
			}
			if health.Version != "4.10.1.0" {
				t.Fatalf("version = %q, want 4.10.1.0", health.Version)
			}
		})
	}
}

func TestEmbyGetSummary_CombinesSystemAndPlayingSessions(t *testing.T) {
	t.Parallel()

	server := newFakeEmby(t)

	for _, suffix := range []string{"", "/emby/"} {
		t.Run("url"+suffix, func(t *testing.T) {
			service := NewEmbyService().(*JellyfinService)
			summary, err := service.GetSummary(t.Context(), server.URL+suffix, "abc123")
			if err != nil {
				t.Fatalf("GetSummary error: %v", err)
			}
			if summary.System.ServerName != "Emby" || summary.System.Version != "4.10.1.0" {
				t.Fatalf("system = %+v, want Emby 4.10.1.0", summary.System)
			}
			if len(summary.Sessions) != 1 || summary.Sessions[0].ID != "1" {
				t.Fatalf("sessions = %+v, want only session 1", summary.Sessions)
			}
		})
	}
}

func TestEmbyGetSummary_ErrorNamesEmby(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	service := NewEmbyService().(*JellyfinService)
	_, err := service.GetSummary(t.Context(), server.URL, "wrong")
	if err == nil {
		t.Fatal("GetSummary error = nil, want error")
	}
	if strings.Contains(err.Error(), "jellyfin") {
		t.Fatalf("error = %q, want no mention of jellyfin", err)
	}
}
