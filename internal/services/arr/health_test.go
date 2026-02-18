// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/services/core"
)

type testArrHealthChecker struct {
	updateCalls int32
	updateValue bool
	updateErr   error
}

func (t *testArrHealthChecker) GetSystemStatus(_ context.Context, _, _ string) (string, error) {
	return "1.0.0", nil
}

func (t *testArrHealthChecker) CheckForUpdates(_ context.Context, _, _ string) (bool, error) {
	atomic.AddInt32(&t.updateCalls, 1)
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
	_, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker)
	if err != nil {
		t.Fatalf("performHealthCheck failed: %v", err)
	}

	if calls := atomic.LoadInt32(&checker.updateCalls); calls != 0 {
		t.Fatalf("expected no update-check call when cached, got %d", calls)
	}
}

func TestPerformHealthCheck_CachesAsyncUpdateResult(t *testing.T) {
	t.Parallel()

	server := newHealthServer(t)
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{updateValue: true}

	health, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker)
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

	if calls := atomic.LoadInt32(&checker.updateCalls); calls != 1 {
		t.Fatalf("expected exactly one update-check call, got %d", calls)
	}
}

func TestPerformHealthCheck_CachesFallbackOnUpdateError(t *testing.T) {
	t.Parallel()

	server := newHealthServer(t)
	defer server.Close()

	serviceCore := newServiceCore(t)
	checker := &testArrHealthChecker{updateErr: errors.New("upstream timeout")}

	_, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker)
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

	_, err = performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker)
	if err != nil {
		t.Fatalf("second performHealthCheck failed: %v", err)
	}

	if calls := atomic.LoadInt32(&checker.updateCalls); calls != 1 {
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

	health, err := performHealthCheck(context.Background(), serviceCore, server.URL, "apikey", checker)
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
