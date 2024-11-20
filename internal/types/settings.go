// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

// ServiceConfiguration is the database model
type ServiceConfiguration struct {
	ID          int64       `json:"-"` // Hide ID from JSON response
	Type        ServiceType `json:"serviceType"`
	InstanceID  string      `json:"instanceId" gorm:"uniqueIndex"`
	DisplayName string      `json:"displayName"`
	URL         string      `json:"url"`
	APIKey      string      `json:"apiKey,omitempty"`
	AccessURL   string      `json:"accessUrl,omitempty"`
}
