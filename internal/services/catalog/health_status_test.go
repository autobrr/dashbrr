// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package catalog_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/catalog"
)

type statusRow struct {
	code        int
	contentType string
	want        string
}

var statusErrorRows = []statusRow{
	{http.StatusUnauthorized, "application/json", "Invalid API key"},
	{http.StatusForbidden, "text/plain", "Access forbidden"},
	{http.StatusNotFound, "text/html", "Service endpoint not found"},
	{http.StatusServiceUnavailable, "text/html", "Service is temporarily unavailable (503 Service Unavailable)"},
	{http.StatusInternalServerError, "text/plain", "Server returned Internal Server Error (500)"},
}

func TestCheckHealthShowsFriendlyStatusMessage(t *testing.T) {
	t.Parallel()

	jsonErrorRow := statusRow{http.StatusInternalServerError, "application/json", "Server returned Internal Server Error (500)"}
	for _, serviceType := range []string{"sonarr", "radarr", "lidarr", "readarr", "whisparr", "prowlarr", "maintainerr"} {
		for _, row := range append(statusErrorRows, jsonErrorRow) {
			checkHealthMessage(t, serviceType, row, row.want)
		}
	}
	for _, row := range statusErrorRows {
		checkHealthMessage(t, "overseerr", row, "Service error:\n• "+row.want)
	}
}

func checkHealthMessage(t *testing.T, serviceType string, row statusRow, want string) {
	t.Helper()
	t.Run(fmt.Sprintf("%s/%d/%s", serviceType, row.code, row.contentType), func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", row.contentType)
			w.WriteHeader(row.code)
			_, _ = w.Write([]byte(`{"error":"raw upstream detail"}`))
		}))
		defer srv.Close()

		service := catalog.Registry{}.CreateService(serviceType)
		health, _ := service.CheckHealth(t.Context(), srv.URL, "wrong-key")
		if health.Status != "error" {
			t.Errorf("status = %q, want %q", health.Status, "error")
		}
		if health.Message != want {
			t.Errorf("message = %q, want %q", health.Message, want)
		}
	})
}
