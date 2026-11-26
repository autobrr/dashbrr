// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import "testing"

func TestServiceRequiresAPIKey(t *testing.T) {
	t.Parallel()

	if serviceRequiresAPIKey("general") {
		t.Fatalf("general should not require api key")
	}
	if serviceRequiresAPIKey("traefik") {
		t.Fatalf("traefik should not require api key")
	}
	if !serviceRequiresAPIKey("radarr") {
		t.Fatalf("radarr should require api key")
	}
}

func TestServiceAllowsURLCredentials(t *testing.T) {
	t.Parallel()

	if !serviceAllowsURLCredentials("uptimekuma") {
		t.Fatalf("uptimekuma should allow url credentials")
	}
	if !serviceAllowsURLCredentials("nzbget") {
		t.Fatalf("nzbget should allow url credentials")
	}
	if serviceAllowsURLCredentials("traefik") {
		t.Fatalf("traefik should not rely on url credentials-only policy")
	}
}

func TestServiceHasCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		serviceType string
		url         string
		apiKey      string
		want        bool
	}{
		{"sonarr", "http://sonarr.test", "key", true},
		{"sonarr", "http://sonarr.test", "", false},
		{"sonarr", "http://user:pass@sonarr.test", "", false},
		{"general", "http://site.test", "", true},
		{"traefik", "http://traefik.test", "", true},
		{"nzbget", "http://user:pass@nzbget.test", "", true},
		{"uptimekuma", "http://user:pass@kuma.test", "", true},
		{"nzbget", "http://user@nzbget.test", "", false},
		{"nzbget", "http://user:@nzbget.test", "", false},
		{"nzbget", "http://nzbget.test", "", false},
		{"nzbget", "http://nzbget.test", "user:pass", true},
	}
	for _, tc := range cases {
		if got := serviceHasCredentials(tc.serviceType, tc.url, tc.apiKey); got != tc.want {
			t.Errorf("serviceHasCredentials(%q, %q, %q) = %v, want %v", tc.serviceType, tc.url, tc.apiKey, got, tc.want)
		}
	}
}
