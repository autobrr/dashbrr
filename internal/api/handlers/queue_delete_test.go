// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
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

func TestDeleteQueueItem_RefreshesPoller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	tests := []struct {
		service    string
		newHandler func(*database.DB, *Poller) gin.HandlerFunc
	}{
		{"sonarr", func(db *database.DB, p *Poller) gin.HandlerFunc { return NewSonarrHandler(db, p).DeleteQueueItem }},
		{"radarr", func(db *database.DB, p *Poller) gin.HandlerFunc { return NewRadarrHandler(db, p).DeleteQueueItem }},
		{"lidarr", func(db *database.DB, p *Poller) gin.HandlerFunc { return NewLidarrHandler(db, p).DeleteQueueItem }},
		{"readarr", func(db *database.DB, p *Poller) gin.HandlerFunc { return NewReadarrHandler(db, p).DeleteQueueItem }},
		{"whisparr", func(db *database.DB, p *Poller) gin.HandlerFunc { return NewWhisparrHandler(db, p).DeleteQueueItem }},
	}

	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			db, cleanup := setupUIPreferencesTestDB(t)
			defer cleanup()

			instanceID := tt.service + "-1"
			if err := db.CreateService(t.Context(), &models.ServiceConfiguration{
				InstanceID: instanceID,
				URL:        upstream.URL,
				APIKey:     "key",
			}); err != nil {
				t.Fatalf("create service: %v", err)
			}

			poller := NewPoller(db, nil)
			r := gin.New()
			r.DELETE("/queue/:id", tt.newHandler(db, poller))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/queue/7?instanceId="+instanceID, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
			}
			assertRefreshed(t, poller, instanceID)
		})
	}
}

func assertRefreshed(t *testing.T, poller *Poller, instanceID string) {
	t.Helper()
	select {
	case req := <-poller.refreshCh:
		if req.instanceID != instanceID {
			t.Fatalf("Refresh instanceID = %q, want %q", req.instanceID, instanceID)
		}
	default:
		t.Fatal("expected Poller.Refresh after a successful action")
	}
}
