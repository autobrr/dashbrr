// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import "strings"

func serviceRequiresAPIKey(serviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "general", "traefik":
		return false
	default:
		return true
	}
}

func serviceAllowsURLCredentials(serviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "nzbget", "uptimekuma":
		return true
	default:
		return false
	}
}
