// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import "testing"

func TestIsAuthBypassEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "on", value: "on", want: true},
		{name: "false", value: "false", want: false},
	}

	for _, tt := range tests {
		t.Setenv("DASHBRR_AUTH_BYPASS", tt.value)
		if got := IsAuthBypassEnabled(); got != tt.want {
			t.Fatalf("%s: IsAuthBypassEnabled() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
