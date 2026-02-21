// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import "net/http"

// normalizeUpstreamStatus maps upstream HTTP responses (service API calls) to
// status codes that won't be confused with dashbrr's own auth layer.
//
// Rule of thumb: 401/403 are reserved for user auth; upstream auth failures
// should be surfaced as 502.
func normalizeUpstreamStatus(code int) int {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return http.StatusBadGateway
	default:
		return code
	}
}
