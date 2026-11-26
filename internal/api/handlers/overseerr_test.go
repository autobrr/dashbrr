// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/models"
)

func TestUpdateRequestStatus_RefreshesPoller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	if err := db.CreateService(t.Context(), &models.ServiceConfiguration{
		InstanceID: "overseerr-1",
		URL:        upstream.URL,
		APIKey:     "key",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	poller := NewPoller(db, nil)
	r := gin.New()
	r.POST("/services/:instanceId/overseerr/request/:requestId/:status", NewOverseerrHandler(db, poller).UpdateRequestStatus)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/services/overseerr-1/overseerr/request/5/2", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
	assertRefreshed(t, poller, "overseerr-1")
}

func TestUpdateRequestStatus_RetriesTransientFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	if err := db.CreateService(t.Context(), &models.ServiceConfiguration{
		InstanceID: "overseerr-1",
		URL:        upstream.URL,
		APIKey:     "key",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	r := gin.New()
	r.POST("/services/:instanceId/overseerr/request/:requestId/:status", NewOverseerrHandler(db, NewPoller(db, nil)).UpdateRequestStatus)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/services/overseerr-1/overseerr/request/5/3", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d after one failed attempt: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want 2", got)
	}
}
