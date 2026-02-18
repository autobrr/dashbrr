// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
)

func TestQueueDeleteOptionsFromQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?removeFromClient=true&blocklist=true&skipRedownload=false&changeCategory=true", nil)

	opts := queueDeleteOptionsFromQuery(c)
	if !opts.RemoveFromClient || !opts.Blocklist || opts.SkipRedownload || !opts.ChangeCategory {
		t.Fatalf("queueDeleteOptionsFromQuery parsed unexpected flags: %+v", opts)
	}
}

func TestHandleQueueDeleteError_ServiceNotConfigured(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handled := handleQueueDeleteError(c, NewServiceNotConfigured("sonarr"), "Sonarr", "sonarr-1", "123")
	if !handled {
		t.Fatal("expected handled=true")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleQueueDeleteError_ArrHTTPCode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := &arr.ErrArr{Service: "sonarr", Op: "delete_queue_item", HttpCode: http.StatusNotFound}
	handled := handleQueueDeleteError(c, err, "Sonarr", "sonarr-1", "123")
	if !handled {
		t.Fatal("expected handled=true")
	}
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestHandleQueueDeleteError_Generic(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handled := handleQueueDeleteError(c, errors.New("boom"), "Radarr", "radarr-1", "123")
	if !handled {
		t.Fatal("expected handled=true")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "Failed to delete queue item: boom" {
		t.Fatalf("error = %q, want %q", body["error"], "Failed to delete queue item: boom")
	}
}

func TestRefreshQueueAfterDelete_BroadcastsFreshData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := cache.NewMemoryStore(ctx, t.TempDir())

	type payload struct {
		Value int `json:"value"`
	}

	var got payload
	called := false

	refreshQueueAfterDelete(
		ctx,
		store,
		resilience.NewCircuitBreaker(5, time.Minute),
		"sonarr:queue:sonarr-1",
		time.Minute,
		2*time.Minute,
		func() (payload, error) {
			return payload{Value: 42}, nil
		},
		func(p *payload) {
			called = true
			got = *p
		},
		"Sonarr",
		"sonarr-1",
	)

	if !called {
		t.Fatal("expected broadcast callback to be called")
	}
	if got.Value != 42 {
		t.Fatalf("broadcast payload value = %d, want 42", got.Value)
	}
}

func TestRefreshQueueAfterDelete_SkipsBroadcastOnFetchError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := cache.NewMemoryStore(ctx, t.TempDir())

	called := false
	refreshQueueAfterDelete(
		ctx,
		store,
		resilience.NewCircuitBreaker(5, time.Minute),
		"radarr:queue:radarr-1",
		time.Minute,
		2*time.Minute,
		func() (map[string]int, error) {
			return nil, errors.New("fetch failed")
		},
		func(_ *map[string]int) {
			called = true
		},
		"Radarr",
		"radarr-1",
	)

	if called {
		t.Fatal("expected broadcast callback to be skipped")
	}
}
