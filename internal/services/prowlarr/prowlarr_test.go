// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package prowlarr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildIndexerStatsURL_UsesRFC3339Window(t *testing.T) {
	now := time.Date(2026, time.February, 19, 12, 0, 0, 0, time.UTC)
	statsURL := buildIndexerStatsURL("http://localhost:9696/", now)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, statsURL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	startDateRaw := req.URL.Query().Get("startDate")
	endDateRaw := req.URL.Query().Get("endDate")
	if startDateRaw == "" || endDateRaw == "" {
		t.Fatalf("expected startDate/endDate query params, got startDate=%q endDate=%q", startDateRaw, endDateRaw)
	}

	startDate, err := time.Parse(time.RFC3339, startDateRaw)
	if err != nil {
		t.Fatalf("startDate should be RFC3339, got %q: %v", startDateRaw, err)
	}

	endDate, err := time.Parse(time.RFC3339, endDateRaw)
	if err != nil {
		t.Fatalf("endDate should be RFC3339, got %q: %v", endDateRaw, err)
	}

	wantStartDate := now.Add(-prowlarrIndexerStatsWindow)
	if !startDate.Equal(wantStartDate) {
		t.Fatalf("unexpected startDate: got %s want %s", startDate, wantStartDate)
	}

	if !endDate.Equal(now) {
		t.Fatalf("unexpected endDate: got %s want %s", endDate, now)
	}

	if !startDate.Before(endDate) {
		t.Fatalf("expected startDate before endDate, got startDate=%s endDate=%s", startDate, endDate)
	}
}

func TestGetIndexerStats_SendsProwlarrCompatibleDateParams(t *testing.T) {
	var seenStartDate string
	var seenEndDate string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/indexerstats" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		seenStartDate = r.URL.Query().Get("startDate")
		seenEndDate = r.URL.Query().Get("endDate")

		startDate, err := time.Parse(time.RFC3339, seenStartDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		endDate, err := time.Parse(time.RFC3339, seenEndDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !startDate.Before(endDate) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"indexers":[]}`))
	}))
	defer server.Close()

	service := &ProwlarrService{}
	stats, err := service.GetIndexerStats(context.Background(), server.URL, "api-key")
	if err != nil {
		t.Fatalf("GetIndexerStats returned error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected stats response, got nil")
	}
	if len(stats.Indexers) != 0 {
		t.Fatalf("expected 0 indexers, got %d", len(stats.Indexers))
	}
	if seenStartDate == "" || seenEndDate == "" {
		t.Fatalf("expected startDate/endDate query params, got startDate=%q endDate=%q", seenStartDate, seenEndDate)
	}
}
