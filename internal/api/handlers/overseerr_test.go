// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"net/http/httptest"
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
