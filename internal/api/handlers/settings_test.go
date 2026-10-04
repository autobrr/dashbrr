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

	"github.com/autobrr/dashbrr/internal/models"
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

func TestSettingsRefuseDiscoveredService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The check runs before any database access. The handler has no database,
	// so a request that reaches the database panics.
	h := &SettingsHandler{}
	router := gin.New()
	router.POST("/settings/:instance", h.SaveSettings)
	router.DELETE("/settings/:instance", h.DeleteSettings)

	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "edit", method: http.MethodPost, body: `{"url":"http://radarr:7878"}`},
		{name: "delete", method: http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/settings/radarr-k8s-media.radarr", strings.NewReader(tt.body))
			router.ServeHTTP(rec, req)

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
		})
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
