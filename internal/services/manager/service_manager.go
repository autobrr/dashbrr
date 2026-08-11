// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package manager

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/seerr"
)

// ServiceManager handles service initialization and data fetching
type ServiceManager struct {
	db    *database.DB
	cache cache.Store
}

// NewServiceManager creates a new service manager instance
func NewServiceManager(db *database.DB, cache cache.Store) *ServiceManager {
	return &ServiceManager{
		db:    db,
		cache: cache,
	}
}

// InitializeService handles initial data fetching for a newly configured service
func (m *ServiceManager) InitializeService(ctx context.Context, config *models.ServiceConfiguration) {
	// Extract service type from instance ID (e.g., "seerr-1" -> "seerr")
	serviceType, ok := models.ServiceTypeFromInstanceID(config.InstanceID)
	if !ok {
		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Skipping initialization - invalid instance id")
		return
	}

	// Skip initialization if URL or API key is missing
	if config.URL == "" || config.APIKey == "" {
		log.Debug().
			Str("type", serviceType).
			Str("instance", config.InstanceID).
			Msg("Skipping initialization - missing URL or API key")
		return
	}

	switch serviceType {
	case "seerr":
		m.initializeSeerr(ctx, config)
	case "plex":
		m.initializePlex(ctx, config)
	// Add other service types here as needed
	// case "radarr":
	//     m.initializeRadarr(ctx, config)
	// case "sonarr":
	//     m.initializeSonarr(ctx, config)
	default:
		log.Debug().
			Str("type", serviceType).
			Str("instance", config.InstanceID).
			Msg("No initialization needed for service type")
	}
}

// initializeSeerr handles Seerr-specific initialization
func (m *ServiceManager) initializeSeerr(ctx context.Context, config *models.ServiceConfiguration) {
	// Check if we already have fresh data in cache
	cacheKey := "seerr:requests:" + config.InstanceID
	var cachedData any
	if err := m.cache.Get(ctx, cacheKey, &cachedData); err == nil {
		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Using cached Seerr data")
		return
	}

	// Create service instance
	service := &seerr.SeerrService{}
	service.SetDB(m.db)

	// Fetch requests in a goroutine
	go func() {
		// Detach from the request lifecycle, but keep values/deadlines if any.
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), core.DefaultTimeout)
		defer cancel()

		stats, err := service.GetRequests(bgCtx, config.URL, config.APIKey)
		if err != nil {
			log.Error().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to fetch initial Seerr requests")
			return
		}

		// Cache the results
		if err := m.cache.Set(bgCtx, cacheKey, stats, 5*time.Minute); err != nil {
			log.Warn().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to cache Seerr requests")
			return
		}

		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Successfully fetched and cached initial Seerr requests")
	}()
}

// initializePlex handles Plex-specific initialization
func (m *ServiceManager) initializePlex(ctx context.Context, config *models.ServiceConfiguration) {
	// Check if we already have fresh data in cache
	cacheKey := "plex:sessions:" + config.InstanceID
	var cachedData any
	if err := m.cache.Get(ctx, cacheKey, &cachedData); err == nil {
		log.Debug().
			Str("instance", config.InstanceID).
			Msg("Using cached Plex sessions data")
		return
	}

	// Create service instance
	service := &plex.PlexService{}
	service.SetDB(m.db)

	// Fetch sessions in a goroutine
	go func() {
		// Detach from the request lifecycle, but keep values/deadlines if any.
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), core.DefaultTimeout)
		defer cancel()

		sessions, err := service.GetSessions(bgCtx, config.URL, config.APIKey)
		if err != nil {
			log.Error().
				Err(err).
				Str("instance", config.InstanceID).
				Msg("Failed to fetch initial Plex sessions")
			return
		}

		// Cache the results with a shorter TTL since sessions are more real-time
		if err := m.cache.Set(bgCtx, cacheKey, sessions, 30*time.Second); err != nil {
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
