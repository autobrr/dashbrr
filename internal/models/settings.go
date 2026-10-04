// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

// ServiceConfiguration is the database model
type ServiceConfiguration struct {
	ID          int64  `json:"-"` // Hide ID from JSON response
	InstanceID  string `json:"instanceId" gorm:"uniqueIndex"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey,omitempty"`
	AccessURL   string `json:"accessUrl,omitempty"`
	// Discovered marks a service that Kubernetes discovery owns. The API sets
	// it on each response. The database does not store it.
	Discovered bool `json:"discovered"`
}
