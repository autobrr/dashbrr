// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package whisparr

import "testing"

// Whisparr V2 is a Sonarr V3 fork, so the service must speak /api/v3 and
// authenticate with X-Api-Key. A wrong default port or an API version copied
// from Lidarr would break these without failing to compile.
func TestNewWhisparrServiceDefaults(t *testing.T) {
	t.Parallel()

	service, ok := NewWhisparrService().(*WhisparrService)
	if !ok {
		t.Fatalf("NewWhisparrService did not return *WhisparrService")
	}

	if got, want := service.Type, "whisparr"; got != want {
		t.Errorf("Type = %q, want %q", got, want)
	}
	if got, want := service.DisplayName, "Whisparr"; got != want {
		t.Errorf("DisplayName = %q, want %q", got, want)
	}
	if got, want := service.DefaultURL, "http://localhost:6969"; got != want {
		t.Errorf("DefaultURL = %q, want %q", got, want)
	}
	if got, want := service.HealthEndpoint, "/api/v3/health"; got != want {
		t.Errorf("HealthEndpoint = %q, want %q", got, want)
	}
}

func TestGetHealthEndpoint(t *testing.T) {
	t.Parallel()

	service := &WhisparrService{}

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"plain", "http://localhost:6969", "http://localhost:6969/api/v3/health"},
		{"trailing slash", "http://localhost:6969/", "http://localhost:6969/api/v3/health"},
		{"repeated trailing slashes", "http://localhost:6969///", "http://localhost:6969/api/v3/health"},
		{"subpath", "https://example.com/whisparr", "https://example.com/whisparr/api/v3/health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := service.GetHealthEndpoint(tt.baseURL); got != tt.want {
				t.Errorf("GetHealthEndpoint(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}
