// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"strings"
)

// ServiceCreator is responsible for creating service instances
type ServiceCreator interface {
	CreateService(serviceType string) ServiceHealthChecker
}

// ServiceRegistry is the default implementation of ServiceCreator
type ServiceRegistry struct{}

// CreateService returns a new service instance based on the service type
func (r *ServiceRegistry) CreateService(serviceType string) ServiceHealthChecker {
	switch strings.ToLower(serviceType) {
	case "autobrr":
		if NewAutobrrService != nil {
			return NewAutobrrService()
		}
	case "radarr":
		if NewRadarrService != nil {
			return NewRadarrService()
		}
	case "sonarr":
		if NewSonarrService != nil {
			return NewSonarrService()
		}
	case "prowlarr":
		if NewProwlarrService != nil {
			return NewProwlarrService()
		}
	case "overseerr":
		if NewOverseerrService != nil {
			return NewOverseerrService()
		}
	case "plex":
		if NewPlexService != nil {
			return NewPlexService()
		}
	case "omegabrr":
		if NewOmegabrrService != nil {
			return NewOmegabrrService()
		}
	case "tailscale":
		if NewTailscaleService != nil {
			return NewTailscaleService()
		}
	case "maintainerr":
		if NewMaintainerrService != nil {
			return NewMaintainerrService()
		}
	case "general":
		if NewGeneralService != nil {
			return NewGeneralService()
		}
	}
	// Return nil for unknown service types
	return nil
}

// NewServiceRegistry creates a new instance of ServiceRegistry
func NewServiceRegistry() ServiceCreator {
	return &ServiceRegistry{}
}
