// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package bazarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/types"
)

func TestGetSummary_FiltersHealthyProvidersAndDisabledSignalR(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/badges":
			_ = json.NewEncoder(w).Encode(types.BazarrBadges{
				Episodes:      100,
				Movies:        25,
				Providers:     1,
				Status:        0,
				SonarrSignalR: "DOWN",
				RadarrSignalR: "LIVE",
				Announcements: 0,
			})
		case "/api/providers":
			_ = json.NewEncoder(w).Encode(types.BazarrProvidersEnvelope{
				Data: []types.BazarrProviderStatus{
					{Name: "provider-a", Status: "Good", Retry: "-"},
					{Name: "provider-b", Status: "Good", Retry: "-"},
					{Name: "provider-c", Status: "Throttled", Retry: "10m"},
				},
			})
		case "/api/system/health":
			_ = json.NewEncoder(w).Encode(types.BazarrHealthIssuesEnvelope{Data: []types.BazarrHealthIssue{}})
		case "/api/system/status":
			_ = json.NewEncoder(w).Encode(types.BazarrSystemStatusEnvelope{
				Data: types.BazarrSystemStatus{
					BazarrVersion: "1.6.3",
					SonarrVersion: "",
					RadarrVersion: "5.14.0",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewBazarrService().(*BazarrService)
	summary, err := svc.GetSummary(context.Background(), server.URL, "secret-key")
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if len(summary.Providers) != 1 || summary.Providers[0].Name != "provider-c" {
		t.Fatalf("Providers = %+v, want only non-Good provider-c", summary.Providers)
	}
	if summary.Badges.SonarrSignalR != "" {
		t.Fatalf("SonarrSignalR = %q, want empty when sonarr_version is empty", summary.Badges.SonarrSignalR)
	}
	if summary.Badges.RadarrSignalR != "LIVE" {
		t.Fatalf("RadarrSignalR = %q, want LIVE when radarr_version is set", summary.Badges.RadarrSignalR)
	}
}

func TestGetSummary_PreservesSignalRWhenSystemStatusFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/badges":
			_ = json.NewEncoder(w).Encode(types.BazarrBadges{
				SonarrSignalR: "DOWN",
				RadarrSignalR: "LIVE",
			})
		case "/api/providers":
			_ = json.NewEncoder(w).Encode(types.BazarrProvidersEnvelope{Data: []types.BazarrProviderStatus{}})
		case "/api/system/health":
			_ = json.NewEncoder(w).Encode(types.BazarrHealthIssuesEnvelope{Data: []types.BazarrHealthIssue{}})
		case "/api/system/status":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewBazarrService().(*BazarrService)
	summary, err := svc.GetSummary(context.Background(), server.URL, "secret-key")
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if summary.Badges.SonarrSignalR != "DOWN" {
		t.Fatalf("SonarrSignalR = %q, want DOWN preserved when status check fails", summary.Badges.SonarrSignalR)
	}
	if summary.Badges.RadarrSignalR != "LIVE" {
		t.Fatalf("RadarrSignalR = %q, want LIVE preserved when status check fails", summary.Badges.RadarrSignalR)
	}
}

func TestGetSummary_DoesNotWaitForHungSystemStatus(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/badges":
			_ = json.NewEncoder(w).Encode(types.BazarrBadges{
				Episodes:      7,
				Providers:     1,
				SonarrSignalR: "DOWN",
				RadarrSignalR: "LIVE",
			})
		case "/api/providers":
			_ = json.NewEncoder(w).Encode(types.BazarrProvidersEnvelope{
				Data: []types.BazarrProviderStatus{
					{Name: "provider-a", Status: "Good", Retry: "-"},
					{Name: "provider-b", Status: "TooManyRequests", Retry: "in 9 minutes"},
				},
			})
		case "/api/system/health":
			_ = json.NewEncoder(w).Encode(types.BazarrHealthIssuesEnvelope{
				Data: []types.BazarrHealthIssue{{Object: "Sonarr", Issue: "unreachable"}},
			})
		case "/api/system/status":
			select {
			case <-release:
			case <-r.Context().Done():
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(t.Context(), 4*bazarrSystemStatusTimeout)
	defer cancel()

	svc := NewBazarrService().(*BazarrService)
	start := time.Now()
	summary, err := svc.GetSummary(ctx, server.URL, "secret-key")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if limit := bazarrSystemStatusTimeout + time.Second; elapsed > limit {
		t.Fatalf("GetSummary took %v, want at most %v while /api/system/status hangs", elapsed, limit)
	}
	if summary.Badges.Episodes != 7 || summary.Badges.SonarrSignalR != "DOWN" || summary.Badges.RadarrSignalR != "LIVE" {
		t.Fatalf("Badges = %+v, want the badges response with SignalR kept", summary.Badges)
	}
	if len(summary.Providers) != 1 || summary.Providers[0].Name != "provider-b" {
		t.Fatalf("Providers = %+v, want only provider-b", summary.Providers)
	}
	if len(summary.HealthIssues) != 1 {
		t.Fatalf("HealthIssues = %+v, want one issue", summary.HealthIssues)
	}
}
