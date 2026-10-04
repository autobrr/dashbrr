// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/manager"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	configCacheKey    = "settings:configurations"
	configCacheTTL    = 5 * time.Minute
	configDebugLogTTL = 30 * time.Second
)

type SettingsHandler struct {
	db             *database.DB
	cache          cache.Store
	serviceManager *manager.ServiceManager
	poller         *Poller
	lastDebugLog   time.Time
}

// serviceConfigResponse prepares a configuration for an API response: it
// removes the API key and marks a discovered service.
func serviceConfigResponse(c models.ServiceConfiguration) models.ServiceConfiguration {
	c.APIKey = ""
	c.Discovered = models.IsDiscoveredInstanceID(c.InstanceID)
	return c
}

const discoveredServiceMessage = "Kubernetes discovery manages this service. Change its annotations to change it."

// refuseDiscoveredService writes an error and returns true when discovery owns
// the service. The next sync overwrites any change.
func refuseDiscoveredService(c *gin.Context, instanceID string) bool {
	if !models.IsDiscoveredInstanceID(instanceID) {
		return false
	}
	c.JSON(http.StatusForbidden, gin.H{"error": discoveredServiceMessage})
	return true
}

func NewSettingsHandler(db *database.DB, cache cache.Store, poller *Poller) *SettingsHandler {
	return &SettingsHandler{
		db:             db,
		cache:          cache,
		serviceManager: manager.NewServiceManager(db, cache),
		poller:         poller,
		lastDebugLog:   time.Now().Add(-configDebugLogTTL), // Initialize to ensure first log happens
	}
}

// InvalidateSettingsCache drops the cached service configurations, so that
// the next settings request reads them from the database.
func InvalidateSettingsCache(ctx context.Context, store cache.Store) error {
	return store.Delete(ctx, configCacheKey)
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	// Try to get configurations from cache
	var configurations []models.ServiceConfiguration
	err := h.cache.Get(ctx, configCacheKey, &configurations)
	if err == nil {
		// Only log debug messages every 30 seconds to reduce spam
		if time.Since(h.lastDebugLog) > configDebugLogTTL {
			for _, config := range configurations {
				log.Debug().
					Str("instance", config.InstanceID).
					Str("display_name", config.DisplayName).
					Msg("Loading configuration from cache")
			}
			log.Info().Int("count", len(configurations)).Msg("Returning cached configurations")
			h.lastDebugLog = time.Now()
		}

		configMap := make(map[string]models.ServiceConfiguration)
		for _, config := range configurations {
			configMap[config.InstanceID] = serviceConfigResponse(config)
		}
		c.JSON(http.StatusOK, configMap)
		return
	}

	// If not in cache, fetch from database
	configurations, err = h.db.GetAllServices(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching configurations")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Cache the configurations
	if err := h.cache.Set(ctx, configCacheKey, configurations, configCacheTTL); err != nil {
		log.Warn().Err(err).Msg("Failed to cache configurations")
	}

	// Log configurations (with rate limiting)
	if time.Since(h.lastDebugLog) > configDebugLogTTL {
		for _, config := range configurations {
			log.Debug().
				Str("instance", config.InstanceID).
				Str("display_name", config.DisplayName).
				Msg("Loading configuration from database")
		}
		log.Info().Int("count", len(configurations)).Msg("Returning fresh configurations")
		h.lastDebugLog = time.Now()
	}

	configMap := make(map[string]models.ServiceConfiguration)
	for _, config := range configurations {
		configMap[config.InstanceID] = serviceConfigResponse(config)
	}
	c.JSON(http.StatusOK, configMap)
}

const invalidServiceURLMessage = "URL must start with http:// or https://"

// serviceURLsValid reports whether the URL, and the access URL when set,
// are absolute http or https URLs with a host.
func serviceURLsValid(config models.ServiceConfiguration) bool {
	return isHTTPURL(config.URL) && (config.AccessURL == "" || isHTTPURL(config.AccessURL))
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	// net/url accepts any digits as a port, but browsers reject ports above 65535.
	p := u.Port()
	_, err = strconv.ParseUint(p, 10, 16)
	return p == "" || err == nil
}

func (h *SettingsHandler) SaveSettings(c *gin.Context) {
	instanceID := c.Param("instance")
	if refuseDiscoveredService(c, instanceID) {
		return
	}

	var config models.ServiceConfiguration
	if err := c.BindJSON(&config); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error binding JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	config.InstanceID = instanceID
	config.URL = strings.TrimRight(config.URL, "/")

	if !serviceURLsValid(config) {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidServiceURLMessage})
		return
	}

	log.Debug().
		Str("instance", instanceID).
		Str("url", config.URL).
		Str("access_url", config.AccessURL).
		Str("display_name", config.DisplayName).
		Bool("api_key_set", config.APIKey != "").
		Msg("Saving configuration")

	// Check if configuration exists
	existing, err := h.db.FindServiceBy(c.Request.Context(), types.FindServiceParams{InstanceID: instanceID})
	if err != nil && err != sql.ErrNoRows {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}

	// API keys are write-only. If omitted on update, keep existing.
	if existing != nil && config.APIKey == "" {
		config.APIKey = existing.APIKey
	}

	var saveErr error
	if existing == nil {
		// Create new configuration
		log.Debug().Str("instance", instanceID).Msg("Creating new configuration")
		saveErr = h.db.CreateService(c.Request.Context(), &config)
	} else {
		// Update existing configuration
		log.Debug().Str("instance", instanceID).Msg("Updating existing configuration")
		saveErr = h.db.UpdateService(c.Request.Context(), &config)
	}

	if saveErr != nil {
		log.Error().Err(saveErr).Str("instance", instanceID).Msg("Error saving configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	// Initialize service data
	h.serviceManager.InitializeService(c.Request.Context(), &config)

	// Invalidate cache
	if err := InvalidateSettingsCache(c.Request.Context(), h.cache); err != nil {
		log.Warn().Err(err).Msg("Failed to delete configuration cache")
	}

	if h.poller != nil {
		h.poller.Refresh(instanceID)
	}

	log.Info().Str("instance", instanceID).Msg("Successfully saved configuration")
	c.JSON(http.StatusOK, serviceConfigResponse(config))
}

func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	instanceID := c.Param("instance")
	if refuseDiscoveredService(c, instanceID) {
		return
	}

	// Check if configuration exists before deleting
	existing, err := h.db.FindServiceBy(c.Request.Context(), types.FindServiceParams{InstanceID: instanceID})
	if err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}

	if existing == nil {
		log.Warn().Str("instance", instanceID).Msg("No configuration found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
		return
	}

	// Delete the configuration
	if err := h.db.DeleteService(c.Request.Context(), instanceID); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error deleting configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete settings"})
		return
	}

	// Invalidate cache
	if err := InvalidateSettingsCache(c.Request.Context(), h.cache); err != nil {
		log.Warn().Err(err).Msg("Failed to delete configuration cache")
	}

	log.Info().Str("instance", instanceID).Msg("Successfully deleted configuration")
	c.JSON(http.StatusOK, gin.H{"message": "Configuration deleted successfully"})
}
