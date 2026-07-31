// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Top-level scalar fields from arbitrary JSON payloads are exposed via
// details.general for display (#87); nested values and the health-check
// status/message keys are not.
func TestCheckHealth_ExposesScalarFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"username": "soup",
			"ratio": "2.34",
			"seeding": 120,
			"can_download": true,
			"status": "ok",
			"message": "hello",
			"nested": {"skip": "me"},
			"list": [1, 2]
		}`))
	}))
	defer server.Close()

	service := NewGeneralService().(*GeneralService)
	health, code := service.CheckHealth(context.Background(), server.URL, "")

	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q", health.Status, "online")
	}

	fields, ok := health.Details["general"].(map[string]any)
	if !ok {
		t.Fatalf("Details[\"general\"] missing, Details = %#v", health.Details)
	}
	for _, key := range []string{"username", "ratio", "seeding", "can_download"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("fields[%q] missing", key)
		}
	}
	for _, key := range []string{"status", "message", "nested", "list"} {
		if _, ok := fields[key]; ok {
			t.Errorf("fields[%q] should be excluded", key)
		}
	}
}
