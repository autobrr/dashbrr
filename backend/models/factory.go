package models

import (
	"strings"
)

// ServiceFactory is responsible for creating service instances
type ServiceFactory interface {
	CreateService(serviceType string) ServiceHealthChecker
}

// DefaultServiceFactory is the default implementation of ServiceFactory
type DefaultServiceFactory struct{}

// CreateService returns a new service instance based on the service type
func (f *DefaultServiceFactory) CreateService(serviceType string) ServiceHealthChecker {
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
	}
	// Return nil for unknown service types
	return nil
}

// NewServiceFactory creates a new instance of DefaultServiceFactory
func NewServiceFactory() ServiceFactory {
	return &DefaultServiceFactory{}
}
