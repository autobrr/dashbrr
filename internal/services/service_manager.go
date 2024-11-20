// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// ServiceManager handles service initialization and data fetching
type ServiceManager struct {
	db    *database.DB
	cache cache.Store

	services map[string]any
}

// NewServiceManager creates a new service manager instance
func NewServiceManager(db *database.DB, cache cache.Store) *ServiceManager {
	return &ServiceManager{
		db:       db,
		cache:    cache,
		services: make(map[string]any),
	}
}

func (m *ServiceManager) InitializeServices(ctx context.Context) error {
	log.Info().Msg("Initializing services...")

	allServices, err := m.db.GetAllServices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get all services")
	}

	for _, service := range allServices {
		log.Info().Msgf("Initializing service %s...", service.DisplayName)

		if err := m.InitializeService(ctx, &service); err != nil {
			log.Error().Err(err).Msgf("Failed to initialize service %s", service.DisplayName)
			//return errors.Wrap(err, "failed to initialize service")
		}
	}

	return nil
}

// InitializeService handles initial data fetching for a newly configured service
func (m *ServiceManager) InitializeService(ctx context.Context, config *domain.ServiceConfiguration) error {
	// Extract service type from instance ID (e.g., "overseerr-1" -> "overseerr")
	//if config.Type == "" {
	//	return errors.New("missing service type")
	//}
	if config.URL == "" {
		return errors.New("missing service URL")
	}
	if config.APIKey == "" {
		return errors.New("missing service API key")
	}

	// try parse type from instanceID
	if config.Type == "" {
		config.Type = config.Type.ParseString(config.InstanceID)

		if config.Type == "" {
			return errors.New("missing service type")
		}
	}

	switch config.Type {
	case domain.ServiceTypeAutobrr:
		svc := NewAutobrrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	case domain.ServiceTypeGeneral:
		svc := NewGeneralService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	case domain.ServiceTypeMaintainerr:
		svc := NewMaintainerrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	case domain.ServiceTypeOmegabrr:
		svc := NewOmegabrrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	case domain.ServiceTypeOverseerr:
		svc := NewOverseerrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc
		//m.initializeOverseerr(ctx, config)

	case domain.ServiceTypePlex:
		svc := NewPlexService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc
	//m.initializePlex(ctx, config)

	case domain.ServiceTypeProwlarr:
		svc := NewProwlarrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	case domain.ServiceTypeRadarr:
		svc := NewRadarrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc
	//     m.initializeRadarr(ctx, config)

	case domain.ServiceTypeSonarr:
		svc := NewSonarrService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc
	//     m.initializeSonarr(ctx, config)

	case domain.ServiceTypeTailscale:
		svc := NewTailscaleService(m.db, m.cache, config)
		m.services[config.InstanceID] = svc

	default:
		log.Debug().
			Str("type", string(config.Type)).
			Str("instance", config.InstanceID).
			Msg("No initialization needed for service type")
		return errors.Errorf("unsupported service type: %s", config.Type)
	}

	return nil
}

// initializeOverseerr handles Overseerr-specific initialization
func (m *ServiceManager) initializeOverseerr(ctx context.Context, config *domain.ServiceConfiguration) {
	// Check if we already have fresh data in cache
	cacheKey := "overseerr:requests:" + config.InstanceID
	var cachedData interface{}
	if err := m.cache.Get(ctx, cacheKey, &cachedData); err == nil {
		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Using cached Overseerr data")
		return
	}

	// Create service instance
	service := &OverseerrService{}
	//service.SetDB(m.db)

	// Fetch requests in a goroutine
	go func() {
		stats, err := service.GetRequests(ctx, config.URL, config.APIKey)
		if err != nil {
			log.Error().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to fetch initial Overseerr requests")
			return
		}

		// Cache the results
		if err := m.cache.Set(ctx, cacheKey, stats, 5*time.Minute); err != nil {
			log.Warn().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to cache Overseerr requests")
			return
		}

		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Successfully fetched and cached initial Overseerr requests")
	}()
}

// initializePlex handles Plex-specific initialization
func (m *ServiceManager) initializePlex(ctx context.Context, config *domain.ServiceConfiguration) {
	// Check if we already have fresh data in cache
	cacheKey := "plex:sessions:" + config.InstanceID
	var cachedData interface{}
	if err := m.cache.Get(ctx, cacheKey, &cachedData); err == nil {
		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Using cached Plex sessions data")
		return
	}

	// Create service instance
	service := &PlexService{}
	//service.SetDB(m.db)

	// Fetch sessions in a goroutine
	go func() {
		sessions, err := service.GetSessions(ctx, config.URL, config.APIKey)
		if err != nil {
			log.Error().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to fetch initial Plex sessions")
			return
		}

		// Cache the results with a shorter TTL since sessions are more real-time
		if err := m.cache.Set(ctx, cacheKey, sessions, 30*time.Second); err != nil {
			log.Warn().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to cache Plex sessions")
			return
		}

		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Successfully fetched and cached initial Plex sessions")
	}()
}

func (m *ServiceManager) GetServiceHealthChecker(instanceID string) (ServiceHealthChecker, error) {
	log.Info().Str("instanceID", instanceID).Msg("ServiceManager: GetServiceHealthChecker")
	svc, ok := m.services[instanceID]
	if !ok {
		return nil, errors.New("service not found")
	}

	return svc.(ServiceHealthChecker), nil
}

func (m *ServiceManager) GetService(instanceID string) (any, error) {
	log.Info().Str("service", instanceID).Msg("ServiceManager: GetService")
	svc, ok := m.services[instanceID]
	if !ok {
		return nil, errors.New("service not found")
	}

	return svc, nil
}
