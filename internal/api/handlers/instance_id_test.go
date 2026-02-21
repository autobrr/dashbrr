// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireInstanceID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		prefix         string
		serviceName    string
		wantOK         bool
		wantID         string
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "missing instance id",
			query:          "",
			prefix:         "sonarr",
			serviceName:    "Sonarr",
			wantOK:         false,
			wantStatusCode: http.StatusBadRequest,
			wantError:      "instanceId is required",
		},
		{
			name:           "invalid prefix",
			query:          "instanceId=radarr-1",
			prefix:         "sonarr",
			serviceName:    "Sonarr",
			wantOK:         false,
			wantStatusCode: http.StatusBadRequest,
			wantError:      "Invalid Sonarr instance ID",
		},
		{
			name:        "valid instance id",
			query:       "instanceId=sonarr-1",
			prefix:      "sonarr",
			serviceName: "Sonarr",
			wantOK:      true,
			wantID:      "sonarr-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req := httptest.NewRequest(http.MethodGet, "/api/test?"+tt.query, nil)
			c.Request = req

			gotID, gotOK := requireInstanceID(c, tt.prefix, tt.serviceName)
			if gotOK != tt.wantOK {
				t.Fatalf("requireInstanceID() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Fatalf("requireInstanceID() id = %q, want %q", gotID, tt.wantID)
			}

			if tt.wantOK {
				return
			}

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatusCode)
			}

			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to unmarshal response body: %v", err)
			}
			if payload["error"] != tt.wantError {
				t.Fatalf("error message = %q, want %q", payload["error"], tt.wantError)
			}
		})
	}
}

func TestRequireInstanceIDWithMissingMessage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request = req

	gotID, gotOK := requireInstanceIDWithMissingMessage(
		c,
		"autobrr",
		"Autobrr",
		"Instance ID is required",
	)

	if gotOK {
		t.Fatal("requireInstanceIDWithMissingMessage() ok = true, want false")
	}
	if gotID != "" {
		t.Fatalf("requireInstanceIDWithMissingMessage() id = %q, want empty", gotID)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if payload["error"] != "Instance ID is required" {
		t.Fatalf("error message = %q, want %q", payload["error"], "Instance ID is required")
	}
}
