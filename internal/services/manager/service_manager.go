// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package manager

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
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
	// Extract service type from instance ID (e.g., "overseerr-1" -> "overseerr")
	serviceType := strings.Split(config.InstanceID, "-")[0]

	// Skip initialization if URL or API key is missing
	if config.URL == "" || config.APIKey == "" {
		log.Debug().
			Str("type", serviceType).
			Str("instance", config.InstanceID).
			Msg("Skipping initialization - missing URL or API key")
		return
	}

	switch serviceType {
	case "overseerr":
		m.initializeOverseerr(ctx, config)
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

// initializeOverseerr handles Overseerr-specific initialization
func (m *ServiceManager) initializeOverseerr(ctx context.Context, config *models.ServiceConfiguration) {
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
	service := &overseerr.OverseerrService{}
	service.SetDB(m.db)

	// Fetch requests in a goroutine
	go func() {
		stats, err := service.GetRequests(config.URL, config.APIKey)
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
func (m *ServiceManager) initializePlex(ctx context.Context, config *models.ServiceConfiguration) {
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
	service := &plex.PlexService{}
	service.SetDB(m.db)

	// Fetch sessions in a goroutine
	go func() {
		sessions, err := service.GetSessions(config.URL, config.APIKey)
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
