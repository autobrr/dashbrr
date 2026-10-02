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

func TestArrQueueDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	db, cleanup := setupUIPreferencesTestDB(t)
	t.Cleanup(cleanup)
	for _, instanceID := range []string{"sonarr-1", "radarr-1", "lidarr-1", "readarr-1", "whisparr-1", "plex-1"} {
		if err := db.CreateService(t.Context(), &models.ServiceConfiguration{
			InstanceID: instanceID,
			URL:        upstream.URL,
			APIKey:     "key",
		}); err != nil {
			t.Fatalf("create service: %v", err)
		}
	}

	poller := NewPoller(db, nil)
	r := gin.New()
	r.DELETE("/api/arr/queue/:id", NewArrQueueHandler(db, poller).DeleteQueueItem)

	deleteQueueItem := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/arr/queue/7"+query, nil))
		return w
	}

	for _, app := range arr.Apps {
		t.Run(app.Name, func(t *testing.T) {
			instanceID := app.Name + "-1"
			w := deleteQueueItem("?instanceId=" + instanceID)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
			}
			if want := "/api/" + app.APIVersion + "/queue/7"; gotPath != want {
				t.Fatalf("upstream path = %q, want %q", gotPath, want)
			}
			assertRefreshed(t, poller, instanceID)
		})
	}

	errorCases := []struct {
		name     string
		query    string
		wantCode int
		wantErr  string
	}{
		{name: "missing instance", query: "", wantCode: http.StatusBadRequest, wantErr: "instanceId is required"},
		{name: "unknown instance", query: "?instanceId=sonarr-9", wantCode: http.StatusNotFound, wantErr: "sonarr-9 is not configured"},
		{name: "not an arr app", query: "?instanceId=plex-1", wantCode: http.StatusBadRequest, wantErr: "instance is not an *arr app"},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			w := deleteQueueItem(tt.query)
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tt.wantErr {
				t.Fatalf("error = %q, want %q", body["error"], tt.wantErr)
			}
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
