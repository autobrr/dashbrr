// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/url"
	"strings"
)

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

func serviceHasCredentials(serviceType, rawURL, apiKey string) bool {
	if !serviceRequiresAPIKey(serviceType) || apiKey != "" {
		return true
	}
	return serviceAllowsURLCredentials(serviceType) && urlHasUserCredentials(rawURL)
}

func urlHasUserCredentials(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.User == nil {
		return false
	}
	username := strings.TrimSpace(u.User.Username())
	password, _ := u.User.Password()
	return username != "" && strings.TrimSpace(password) != ""
}
