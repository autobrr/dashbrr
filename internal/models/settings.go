// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

// ServiceConfiguration is the database model
type ServiceConfiguration struct {
	ID          int64  `json:"-"` // Hide ID from JSON response
	InstanceID  string `json:"instanceId" gorm:"uniqueIndex"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	AccessURL   string `json:"accessUrl"`
	APIKey      string `json:"apiKey,omitempty"`
}
