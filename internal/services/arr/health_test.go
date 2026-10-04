// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// TestMain starts the global cache, as serve does, so ServiceCore can cache
// update results.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dashbrr-arr-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if _, err := cache.InitCache(context.Background(), cache.Config{DataDir: dir}); err != nil {
		panic(err)
	}
	m.Run()
}

type testArrHealthChecker struct {
	updateCalls atomic.Int32
	updateValue bool
	updateErr   error
}

func (t *testArrHealthChecker) GetSystemStatus(_ context.Context, _, _ string) (string, error) {
	return "1.0.0", nil
}

func (t *testArrHealthChecker) CheckForUpdates(_ context.Context, _, _ string) (bool, error) {
	t.updateCalls.Add(1)
	if t.updateErr != nil {
		return false, t.updateErr
	}
	return t.updateValue, nil
}

func (t *testArrHealthChecker) GetHealthEndpoint(baseURL string) string {
	return baseURL + "/api/v3/health"
}

func waitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return condition()
}

func newHealthServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
}

func newServiceCore(t *testing.T) *core.ServiceCore {
	t.Helper()
	return &core.ServiceCore{}
}

func TestPerformHealthCheck_SkipsUpdateCheckWhenCached(t *testing.T) {
	t.Parallel()

	server := newHealthServer(t)
	defer server.Close()

	serviceCore := newServiceCore(t)
	if err := serviceCore.CacheUpdateStatus(context.Background(), server.URL, false, time.Minute); err != nil {
		t.Fatalf("failed to seed update cache: %v", err)
	}

	checker := &testArrHealthChecker{updateValue: true}
	_, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}

	if calls := checker.updateCalls.Load(); calls != 0 {
		t.Fatalf("expected no update-check call when cached, got %d", calls)
	}
}

func TestPerformHealthCheck_CachesAsyncUpdateResult(t *testing.T) {
	t.Parallel()

	server := newHealthServer(t)
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{updateValue: true}

	health, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}
	if health.UpdateAvailable {
		t.Fatalf("expected first response to use cached update=false before async refresh")
	}

	ok := waitForCondition(2*time.Second, func() bool {
		got, found := serviceCore.GetUpdateStatusFromCacheWithFound(context.Background(), server.URL)
		return found && got
	})
	if !ok {
		t.Fatalf("expected async update status to be cached as true")
	}

	if calls := checker.updateCalls.Load(); calls != 1 {
		t.Fatalf("expected exactly one update-check call, got %d", calls)
	}
}

func TestPerformHealthCheck_CachesFallbackOnUpdateError(t *testing.T) {
	t.Parallel()

	server := newHealthServer(t)
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{updateErr: errors.New("upstream timeout")}

	_, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}

	ok := waitForCondition(2*time.Second, func() bool {
		_, found := serviceCore.GetUpdateStatusFromCacheWithFound(context.Background(), server.URL)
		return found
	})
	if !ok {
		t.Fatalf("expected fallback update status to be cached on error")
	}

	_, err = performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("second performHealthCheck failed: %v", err)
	}

	if calls := checker.updateCalls.Load(); calls != 1 {
		t.Fatalf("expected cached fallback to prevent repeat update checks, got %d calls", calls)
	}
}

func TestPerformHealthCheck_DeduplicatesWarningMessages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"source":"IndexerLongTermStatusCheck","type":"warning","message":"Indexers unavailable due to failures for more than 6 hours: MyAnonamouse"},
			{"source":"IndexerLongTermStatusCheck","type":"warning","message":"Indexers unavailable due to failures for more than 6 hours: MyAnonamouse"}
		]`))
	}))
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{}

	health, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}

	if health.Status != "warning" {
		t.Fatalf("health status = %q, want %q", health.Status, "warning")
	}

	warningLine := "[IndexerLongTermStatusCheck] Indexers unavailable due to failures for more than 6 hours: MyAnonamouse"
	if count := strings.Count(health.Message, warningLine); count != 1 {
		t.Fatalf("warning count = %d, want 1; message=%q", count, health.Message)
	}
}

func TestPerformHealthCheck_DeduplicatesWarningMessagesAfterWhitespaceNormalization(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"source":"IndexerLongTermStatusCheck","type":"warning","message":"Indexers unavailable due to failures for more than 6 hours: MyAnonamouse"},
			{"source":" IndexerLongTermStatusCheck ","type":"warning","message":"Indexers   unavailable due to failures for more than 6 hours: MyAnonamouse"}
		]`))
	}))
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{}

	health, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker, nil)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}

	if health.Status != "warning" {
		t.Fatalf("health status = %q, want %q", health.Status, "warning")
	}

	warningLine := "[IndexerLongTermStatusCheck] Indexers unavailable due to failures for more than 6 hours: MyAnonamouse"
	if count := strings.Count(health.Message, warningLine); count != 1 {
		t.Fatalf("warning count = %d, want 1; message=%q", count, health.Message)
	}
}

func TestPerformHealthCheck_IgnoredHealthChecks(t *testing.T) {
	t.Parallel()

	const removed = `{"source":"RemovedSeriesCheck","type":"warning","message":"Series removed from TheTVDB"}`
	const indexer = `{"source":"IndexerStatusCheck","type":"warning","message":"Indexers unavailable"}`

	tests := []struct {
		name        string
		body        string
		ignored     []string
		wantStatus  string
		wantMessage string
	}{
		{
			name:        "empty list",
			body:        "[" + removed + "]",
			wantStatus:  "warning",
			wantMessage: "[RemovedSeriesCheck] Series removed from TheTVDB",
		},
		{
			name:        "one suppressed source",
			body:        "[" + removed + "]",
			ignored:     []string{"RemovedSeriesCheck"},
			wantStatus:  "online",
			wantMessage: "Healthy",
		},
		{
			name:        "one suppressed and one kept source",
			body:        "[" + removed + "," + indexer + "]",
			ignored:     []string{"RemovedSeriesCheck"},
			wantStatus:  "warning",
			wantMessage: "[IndexerStatusCheck] Indexers unavailable",
		},
		{
			name:        "case and outer spaces ignored",
			body:        "[" + removed + "," + indexer + "]",
			ignored:     []string{"  removedseriescheck "},
			wantStatus:  "warning",
			wantMessage: "[IndexerStatusCheck] Indexers unavailable",
		},
		{
			name:        "all sources suppressed",
			body:        "[" + removed + "," + indexer + "]",
			ignored:     []string{"RemovedSeriesCheck", "INDEXERSTATUSCHECK"},
			wantStatus:  "online",
			wantMessage: "Healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			checker := &testArrHealthChecker{}
			health, err := performHealthCheck(t.Context(), newServiceCore(t), server.URL, "apikey", checker, newIgnoredSet(tt.ignored))
			if err != nil {
				t.Fatalf("performHealthCheck failed: %v", err)
			}
			if health.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", health.Status, tt.wantStatus)
			}
			if health.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", health.Message, tt.wantMessage)
			}
		})
	}
}
