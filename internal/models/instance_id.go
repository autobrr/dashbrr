// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"regexp"
	"strings"
)

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

// kubernetesInstanceIDPattern matches the instance ID of a discovered service:
// <type>-k8s-<namespace>.<service>. Kubernetes names cannot contain dots, so
// IDs from the UI, from a file, or from before #145 do not match.
var kubernetesInstanceIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+-k8s-[a-z0-9-]+\.[a-z0-9-]+$`)

// IsDiscoveredInstanceID reports whether Kubernetes discovery owns the service
// with this instance ID.
func IsDiscoveredInstanceID(instanceID string) bool {
	return kubernetesInstanceIDPattern.MatchString(instanceID)
}
