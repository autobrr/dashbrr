// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"time"
)

// Service represents a configured service instance
type Service struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	URL            string `json:"url"`
	APIKey         string `json:"apiKey,omitempty"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName,omitempty"`
	HealthEndpoint string `json:"healthEndpoint,omitempty"`
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status          string    `json:"status"`
	ResponseTime    int64     `json:"responseTime"`
	LastChecked     time.Time `json:"lastChecked"`
	Message         string    `json:"message,omitempty"`
	Version         string    `json:"version,omitempty"`
	UpdateAvailable bool      `json:"updateAvailable,omitempty"`
	ServiceID       string    `json:"serviceId"`
}

// ServiceHealthChecker defines the interface for service health checking
type ServiceHealthChecker interface {
	CheckHealth(url, apiKey string) (ServiceHealth, int)
}

// Service creation function types
var (
	NewAutobrrService     func() ServiceHealthChecker
	NewRadarrService      func() ServiceHealthChecker
	NewSonarrService      func() ServiceHealthChecker
	NewProwlarrService    func() ServiceHealthChecker
	NewOverseerrService   func() ServiceHealthChecker
	NewPlexService        func() ServiceHealthChecker
	NewOmegabrrService    func() ServiceHealthChecker
	NewTailscaleService   func() ServiceHealthChecker
	NewMaintainerrService func() ServiceHealthChecker
	NewGeneralService     func() ServiceHealthChecker
)
