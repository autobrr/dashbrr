// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package traefik

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthHeaderFromAPIKey(t *testing.T) {
	t.Parallel()

	if got := authHeaderFromAPIKey(""); got != "" {
		t.Fatalf("empty token header = %q, want empty", got)
	}

	if got := authHeaderFromAPIKey("bearer abc"); got != "bearer abc" {
		t.Fatalf("bearer passthrough = %q, want %q", got, "bearer abc")
	}

	wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if got := authHeaderFromAPIKey("user:pass"); got != wantBasic {
		t.Fatalf("basic header = %q, want %q", got, wantBasic)
	}

	if got := authHeaderFromAPIKey("token123"); got != "Bearer token123" {
		t.Fatalf("token header = %q, want %q", got, "Bearer token123")
	}
}

func TestGetSummary_FetchesOverviewAndIssueRouters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token123" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer token123")
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/overview":
			_, _ = w.Write([]byte(`{
				"http":{"routers":{"total":12,"warnings":2,"errors":1},"services":{"total":9,"warnings":1,"errors":0},"middlewares":{"total":7,"warnings":0,"errors":0}},
				"tcp":{"routers":{"total":2,"warnings":0,"errors":0},"services":{"total":2,"warnings":0,"errors":0},"middlewares":{"total":1,"warnings":0,"errors":0}},
				"udp":{"routers":{"total":0,"warnings":0,"errors":0},"services":{"total":0,"warnings":0,"errors":0}},
				"features":{"tracing":"jaeger","metrics":"Prometheus","accessLog":true},
				"providers":["Docker","File"]
			}`))
		case "/api/http/routers":
			switch r.URL.Query().Get("status") {
			case "warning":
				_, _ = w.Write([]byte(`[
					{"name":"warn-router@docker","provider":"docker","status":"warning","rule":"Host(` + "`warn.example`" + `)","service":"warn@docker","entryPoints":["websecure"]}
				]`))
			case "disabled":
				_, _ = w.Write([]byte(`[
					{"name":"disabled-router@docker","provider":"docker","status":"disabled","rule":"Host(` + "`down.example`" + `)","service":"down@docker","entryPoints":["websecure"]}
				]`))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewTraefikService().(*TraefikService)
	summary, err := service.GetSummary(context.Background(), server.URL, "token123")
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if summary.Overview.HTTP.Routers == nil || summary.Overview.HTTP.Routers.Total != 12 {
		t.Fatalf("http router total = %#v, want 12", summary.Overview.HTTP.Routers)
	}
	if len(summary.IssueRouters) != 2 {
		t.Fatalf("issue router count = %d, want 2", len(summary.IssueRouters))
	}
	if summary.IssueRouters[0].Status != "disabled" {
		t.Fatalf("issueRouters[0].status = %q, want disabled first", summary.IssueRouters[0].Status)
	}
}

func TestCheckHealth_UsesVersionEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Fatalf("path = %q, want /api/version", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"3.4.1"}`))
	}))
	defer server.Close()

	service := NewTraefikService().(*TraefikService)
	health, statusCode := service.CheckHealth(context.Background(), server.URL, "")
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want 200", statusCode)
	}
	if health.Status != "online" {
		t.Fatalf("status = %q, want online", health.Status)
	}
	if health.Version != "3.4.1" {
		t.Fatalf("version = %q, want 3.4.1", health.Version)
	}
}
