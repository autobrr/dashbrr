// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import "strings"

// ServiceTypeFromInstanceID extracts the service type prefix from an instance id
// like "radarr-1" or "general-myhost".
func ServiceTypeFromInstanceID(instanceID string) (string, bool) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return "", false
	}
	t, _, ok := strings.Cut(instanceID, "-")
	if !ok {
		// No dash: treat the whole string as the type.
		t = instanceID
	}
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return "", false
	}
	return t, true
}
