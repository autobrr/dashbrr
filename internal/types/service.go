// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import (
	"strings"
	"time"
)

type ServiceType string

const (
	ServiceTypeAutobrr     ServiceType = "AUTOBRR"
	ServiceTypeRadarr      ServiceType = "RADARR"
	ServiceTypeSonarr      ServiceType = "SONARR"
	ServiceTypeProwlarr    ServiceType = "PROWLARR"
	ServiceTypeOverseerr   ServiceType = "OVERSEERR"
	ServiceTypePlex        ServiceType = "PLEX"
	ServiceTypeOmegabrr    ServiceType = "OMEGABRR"
	ServiceTypeTailscale   ServiceType = "TAILSCALE"
	ServiceTypeMaintainerr ServiceType = "MAINTAINERR"
	ServiceTypeGeneral     ServiceType = "GENERAL"
)

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) FromString(str string) ServiceType {
	if str == "" {
	}
	return ServiceType(str)
}

func (s ServiceType) ParseString(str string) ServiceType {
	if strings.Contains(str, "-") {
		parts := strings.Split(str, "-")
		if len(parts) == 2 {
			return ServiceType(strings.ToUpper(parts[0]))
		}
	}

	return "INVALID"
}

// Service represents a configured service instance
type Service struct {
	ID             string      `json:"id"`
	Type           ServiceType `json:"type"`
	URL            string      `json:"url"`
	AccessURL      string      `json:"accessUrl,omitempty"` // New field for external access URL
	APIKey         string      `json:"apiKey,omitempty"`
	Name           string      `json:"name"`
	DisplayName    string      `json:"displayName,omitempty"`
	HealthEndpoint string      `json:"healthEndpoint,omitempty"`
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status          string                 `json:"status"`
	ResponseTime    int64                  `json:"responseTime"`
	LastChecked     time.Time              `json:"lastChecked"`
	Message         string                 `json:"message,omitempty"`
	Version         string                 `json:"version,omitempty"`
	UpdateAvailable bool                   `json:"updateAvailable,omitempty"`
	ServiceID       string                 `json:"serviceId"`
	Stats           map[string]interface{} `json:"stats,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

//// ServiceHealthChecker defines the interface for service health checking
//type ServiceHealthChecker interface {
//	CheckHealth(ctx context.Context, url, apiKey string) (ServiceHealth, int)
//}

//// Service creation function types
//var (
//	NewAutobrrService     func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewRadarrService      func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewSonarrService      func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewProwlarrService    func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewOverseerrService   func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewPlexService        func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewOmegabrrService    func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewTailscaleService   func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewMaintainerrService func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//	NewGeneralService     func(db *database.DB, cache *cache.Cache) ServiceHealthChecker
//)
