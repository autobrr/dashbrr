// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"
	"github.com/autobrr/dashbrr/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	configCacheKey    = "settings:configurations"
	configCacheTTL    = 5 * time.Minute
	configDebugLogTTL = 30 * time.Second
)

type SettingsHandler struct {
	db             *database.DB
	cache          cache.Store
	serviceManager *services.ServiceManager
	lastDebugLog   time.Time
}

func NewSettingsHandler(db *database.DB, cache cache.Store, serviceManager *services.ServiceManager) *SettingsHandler {
	return &SettingsHandler{
		db:             db,
		cache:          cache,
		serviceManager: serviceManager,
		lastDebugLog:   time.Now().Add(-configDebugLogTTL), // Initialize to ensure first log happens
	}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	// Try to get configurations from cache
	var configurations []domain.ServiceConfiguration
	err := h.cache.Get(c.Request.Context(), configCacheKey, &configurations)
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

		configMap := make(map[string]domain.ServiceConfiguration)
		for _, config := range configurations {
			configMap[config.InstanceID] = config
		}
		c.JSON(http.StatusOK, configMap)
		return
	}

	// If not in cache, fetch from database
	configurations, err = h.db.GetAllServices(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("Error fetching configurations")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Cache the configurations
	if err := h.cache.Set(c.Request.Context(), configCacheKey, configurations, configCacheTTL); err != nil {
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

	configMap := make(map[string]domain.ServiceConfiguration)
	for _, config := range configurations {
		configMap[config.InstanceID] = config
	}
	c.JSON(http.StatusOK, configMap)
}

func (h *SettingsHandler) SaveSettings(c *gin.Context) {
	instanceID := c.Param("instance")

	var config domain.ServiceConfiguration
	if err := c.BindJSON(&config); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error binding JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	config.InstanceID = instanceID
	config.URL = strings.TrimRight(config.URL, "/")

	log.Debug().
		Str("instance", instanceID).
		Interface("config", config).
		Msg("Saving configuration")

	// Check if configuration exists
	existing, err := h.db.FindServiceBy(c.Request.Context(), domain.FindServiceParams{InstanceID: instanceID})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Str("instance", instanceID).Msg("service not found")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
			return
		}

		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}

	var saveErr error
	if existing != nil {
		// If updating, stop health monitoring first
		if err := h.serviceManager.StopMonitoring(instanceID); err != nil {
			log.Error().Err(err).Str("instance", instanceID).Msg("Failed to stop monitoring")
		}

		// Update existing configuration
		log.Debug().Str("instance", instanceID).Msg("Updating existing configuration")
		saveErr = h.db.UpdateService(c.Request.Context(), &config)
	} else {
		// Create new configuration
		log.Debug().Str("instance", instanceID).Msg("Creating new configuration")
		saveErr = h.db.CreateService(c.Request.Context(), &config)
	}

	if saveErr != nil {
		log.Error().Err(saveErr).Str("instance", instanceID).Msg("Error saving configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	// Invalidate cache
	if err := h.cache.Delete(c.Request.Context(), configCacheKey); err != nil {
		log.Warn().Err(err).Msg("Failed to delete configuration cache")
	}

	// Initialize service data
	if err := h.serviceManager.InitializeService(c.Request.Context(), &config); err != nil {
		log.Error().Err(saveErr).Str("instance", instanceID).Msg("could not initialize service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not initialize service"})
		return
	}

	log.Info().Str("instance", instanceID).Msg("Successfully saved configuration")
	c.JSON(http.StatusOK, config)
}

func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	instanceID := c.Param("instance")

	// Check if configuration exists before deleting
	existing, err := h.db.FindServiceBy(c.Request.Context(), domain.FindServiceParams{InstanceID: instanceID})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Str("instance", instanceID).Msg("service not found")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
			return
		}

		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}

	if existing == nil {
		log.Warn().Str("instance", instanceID).Msg("No configuration found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
		return
	}

	// Stop health monitoring before deleting
	if err := h.serviceManager.StopMonitoring(instanceID); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Failed to stop monitoring")
	}

	// Delete the configuration
	// TODO move to method on serviceManager
	if err := h.db.DeleteService(c.Request.Context(), instanceID); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error deleting configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete settings"})
		return
	}

	// Invalidate cache
	if err := h.cache.Delete(c.Request.Context(), configCacheKey); err != nil {
		log.Warn().Err(err).Msg("Failed to delete configuration cache")
	}

	log.Info().Str("instance", instanceID).Msg("Successfully deleted configuration")
	c.JSON(http.StatusOK, gin.H{"message": "Configuration deleted successfully"})
}
