// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import "os"

// IsAuthBypassEnabled returns true when API auth should be bypassed for local troubleshooting.
//
// Env:
//   - DASHBRR_AUTH_BYPASS=true
func IsAuthBypassEnabled() bool {
	v := os.Getenv("DASHBRR_AUTH_BYPASS")
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
