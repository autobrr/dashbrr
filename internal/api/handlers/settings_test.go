// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestServiceURLsValid(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		accessURL string
		want      bool
	}{
		{name: "http url", url: "http://autobrr:7474", want: true},
		{name: "https url with base path", url: "https://host/autobrr", want: true},
		{name: "empty access url", url: "http://autobrr:7474", accessURL: "", want: true},
		{name: "valid access url", url: "http://autobrr:7474", accessURL: "https://autobrr.example", want: true},
		{name: "missing scheme", url: "autobrr:74747", want: false},
		{name: "empty url", url: "", want: false},
		{name: "ftp scheme", url: "ftp://host", want: false},
		{name: "no host", url: "http://", want: false},
		{name: "port without host name", url: "http://:8080", want: false},
		{name: "port above 65535", url: "http://autobrr:74747", want: false},
		{name: "highest port", url: "http://autobrr:65535", want: true},
		{name: "invalid access url", url: "http://autobrr:7474", accessURL: "autobrr:74747", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serviceURLsValid(models.ServiceConfiguration{URL: tt.url, AccessURL: tt.accessURL})
			if got != tt.want {
				t.Fatalf("serviceURLsValid(%q, %q) = %v, want %v", tt.url, tt.accessURL, got, tt.want)
			}
		})
	}
}

func TestSaveSettingsRejectsInvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "url without scheme", body: `{"url":"autobrr:74747"}`},
		{name: "access url without scheme", body: `{"url":"http://autobrr:7474","accessUrl":"autobrr:74747"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The check runs before any database access, so the handler needs no dependencies.
			router := gin.New()
			router.POST("/settings/:instance", (&SettingsHandler{}).SaveSettings)

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/autobrr-1", strings.NewReader(tt.body))
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			var resp struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error != invalidServiceURLMessage {
				t.Fatalf("error = %q, want %q", resp.Error, invalidServiceURLMessage)
			}
		})
	}
}

func TestSettingsDiscoveredService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := t.Context()

	db, err := database.InitDBWithConfig(&database.Config{Driver: "sqlite", Path: t.TempDir() + "/settings.db"})
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// The save starts a Plex fetch, so the URL points at a closed local port.
	discovered := models.ServiceConfiguration{
		InstanceID:  "plex-k8s-media.plex",
		DisplayName: "Plex",
		URL:         "http://127.0.0.1:1",
		APIKey:      "old-token",
	}
	radarr := models.ServiceConfiguration{
		InstanceID:  "radarr-k8s-media.radarr",
		DisplayName: "Movies",
		URL:         "http://radarr.media.svc.cluster.local:7878",
		APIKey:      "annotated-key",
	}
	for _, s := range []*models.ServiceConfiguration{&discovered, &radarr} {
		if err := db.CreateService(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
		s.ID = 0
	}

	h := NewSettingsHandler(db, newBuiltinMemoryStore(t), nil)
	router := gin.New()
	router.POST("/settings/:instance", h.SaveSettings)
	router.DELETE("/settings/:instance", h.DeleteSettings)

	stored := func(id string) *models.ServiceConfiguration {
		t.Helper()
		s, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: id})
		if err != nil {
			t.Fatalf("find %s: %v", id, err)
		}
		if s != nil {
			s.ID = 0
		}
		return s
	}
	wantError := func(rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		var resp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Error != discoveredServiceMessage {
			t.Fatalf("error = %q, want %q", resp.Error, discoveredServiceMessage)
		}
	}

	// A Plex save changes only the token. Discovery owns the other fields.
	rec := serve(t, router, http.MethodPost, "/settings/plex-k8s-media.plex",
		`{"displayName":"Changed","url":"http://other:1","accessUrl":"http://other:2","apiKey":"new-token"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body %s", rec.Code, rec.Body.String())
	}
	want := discovered
	want.APIKey = "new-token"
	if got := stored(want.InstanceID); *got != want {
		t.Fatalf("after save = %+v, want %+v", *got, want)
	}

	// A save without a token keeps the stored token.
	rec = serve(t, router, http.MethodPost, "/settings/plex-k8s-media.plex", `{"url":"http://other:1"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := stored(want.InstanceID); *got != want {
		t.Fatalf("after empty save = %+v, want %+v", *got, want)
	}

	// An annotation owns the key of every other type.
	wantError(serve(t, router, http.MethodPost, "/settings/radarr-k8s-media.radarr", `{"apiKey":"ui-key"}`, nil))
	if got := stored(radarr.InstanceID); *got != radarr {
		t.Fatalf("after radarr save = %+v, want %+v", *got, radarr)
	}

	// Only discovery creates a discovered service.
	wantError(serve(t, router, http.MethodPost, "/settings/plex-k8s-other.plex", `{"url":"http://127.0.0.1:1","apiKey":"k"}`, nil))
	if got := stored("plex-k8s-other.plex"); got != nil {
		t.Fatalf("created %+v", *got)
	}

	// A delete fails, and the record does not change.
	wantError(serve(t, router, http.MethodDelete, "/settings/plex-k8s-media.plex", "", nil))
	if got := stored(want.InstanceID); *got != want {
		t.Fatalf("after delete = %+v, want %+v", *got, want)
	}
}

func TestServiceConfigResponseMarksDiscovered(t *testing.T) {
	tests := []struct {
		instanceID string
		want       bool
	}{
		{instanceID: "radarr-k8s-media.radarr", want: true},
		{instanceID: "radarr-1", want: false},
		{instanceID: "radarr-docker", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.instanceID, func(t *testing.T) {
			got := serviceConfigResponse(models.ServiceConfiguration{InstanceID: tt.instanceID})
			if got.Discovered != tt.want {
				t.Fatalf("Discovered = %v, want %v", got.Discovered, tt.want)
			}
		})
	}
}
