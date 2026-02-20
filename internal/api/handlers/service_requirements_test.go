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
