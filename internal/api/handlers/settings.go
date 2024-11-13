// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/manager"
)

const (
	configCacheKey    = "settings:configurations"
	configCacheTTL    = 5 * time.Minute
	configDebugLogTTL = 30 * time.Second
)

type SettingsHandler struct {
	db             *database.DB
	health         *services.HealthService
	cache          cache.Store
	serviceManager *manager.ServiceManager
	lastDebugLog   time.Time
}

func NewSettingsHandler(db *database.DB, health *services.HealthService, cache cache.Store) *SettingsHandler {
	return &SettingsHandler{
		db:             db,
		health:         health,
		cache:          cache,
		serviceManager: manager.NewServiceManager(db, cache),
		lastDebugLog:   time.Now().Add(-configDebugLogTTL), // Initialize to ensure first log happens
	}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	// Try to get configurations from cache
	var configurations []models.ServiceConfiguration
	err := h.cache.Get(context.Background(), configCacheKey, &configurations)
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
			configMap[config.InstanceID] = config
		}
		c.JSON(http.StatusOK, configMap)
		return
	}

	// If not in cache, fetch from database
	configurations, err = h.db.GetAllServices()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching configurations")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Cache the configurations
	if err := h.cache.Set(context.Background(), configCacheKey, configurations, configCacheTTL); err != nil {
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
		configMap[config.InstanceID] = config
	}
	c.JSON(http.StatusOK, configMap)
}

func (h *SettingsHandler) SaveSettings(c *gin.Context) {
	instanceID := c.Param("instance")

	var config models.ServiceConfiguration
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
	existing, err := h.db.GetServiceByInstanceID(instanceID)
	if err != nil && err != sql.ErrNoRows {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}

	// If updating, stop health monitoring first
	if existing != nil && h.health != nil {
		h.health.StopMonitoring(instanceID)
	}

	var saveErr error
	if existing == nil {
		// Create new configuration
		log.Debug().Str("instance", instanceID).Msg("Creating new configuration")
		saveErr = h.db.CreateService(&config)
	} else {
		// Update existing configuration
		log.Debug().Str("instance", instanceID).Msg("Updating existing configuration")
		saveErr = h.db.UpdateService(&config)
	}

	if saveErr != nil {
		log.Error().Err(saveErr).Str("instance", instanceID).Msg("Error saving configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	// Initialize service data
	h.serviceManager.InitializeService(c.Request.Context(), &config)

	// Invalidate cache
	h.cache.Delete(context.Background(), configCacheKey)

	log.Info().Str("instance", instanceID).Msg("Successfully saved configuration")
	c.JSON(http.StatusOK, config)
}

func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	instanceID := c.Param("instance")

	// Check if configuration exists before deleting
	existing, err := h.db.GetServiceByInstanceID(instanceID)
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

	// Stop health monitoring before deleting
	if h.health != nil {
		log.Debug().Str("instance", instanceID).Msg("Stopping health monitoring")
		h.health.StopMonitoring(instanceID)
	}

	// Delete the configuration
	if err := h.db.DeleteService(instanceID); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error deleting configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete settings"})
		return
	}

	// Invalidate cache
	h.cache.Delete(context.Background(), configCacheKey)

	log.Info().Str("instance", instanceID).Msg("Successfully deleted configuration")
	c.JSON(http.StatusOK, gin.H{"message": "Configuration deleted successfully"})
}
