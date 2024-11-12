// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services"
)

type SettingsHandler struct {
	db     *database.DB
	health *services.HealthService
}

func NewSettingsHandler(db *database.DB, health *services.HealthService) *SettingsHandler {
	return &SettingsHandler{
		db:     db,
		health: health,
	}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	configurations, err := h.db.GetAllServices()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching configurations")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	configMap := make(map[string]models.ServiceConfiguration)
	for _, config := range configurations {
		log.Debug().
			Str("instance", config.InstanceID).
			Str("display_name", config.DisplayName).
			Msg("Loading configuration")
		configMap[config.InstanceID] = config
	}

	log.Info().Int("count", len(configMap)).Msg("Returning configurations")
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
	existing, err := h.db.FindServiceBy(context.Background(), database.FindServiceParams{InstanceID: instanceID})
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

	log.Info().Str("instance", instanceID).Msg("Successfully saved configuration")
	c.JSON(http.StatusOK, config)
}

func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	instanceID := c.Param("instance")

	// Check if configuration exists before deleting
	existing, err := h.db.FindServiceBy(context.Background(), database.FindServiceParams{InstanceID: instanceID})
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

	log.Info().Str("instance", instanceID).Msg("Successfully deleted configuration")
	c.JSON(http.StatusOK, gin.H{"message": "Configuration deleted successfully"})
}
