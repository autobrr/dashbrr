// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/catalog"
	"github.com/autobrr/dashbrr/internal/services/core"
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

func serviceConfigResponse(c models.ServiceConfiguration) models.ServiceConfiguration {
	c.APIKey = ""
	c.Edits = models.AllowedEdits(c.InstanceID)
	return c
}

const discoveredServiceMessage = "Kubernetes discovery manages this service. Change its annotations to change it."

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
	return core.ValidateServiceURL(raw) == nil
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

	switch models.AllowedEdits(instanceID) {
	case models.EditsNone:
		c.JSON(http.StatusForbidden, gin.H{"error": discoveredServiceMessage})
		return
	case models.EditsAll:
	case models.EditsAPIKey:
		discovered, err := h.db.FindServiceBy(c.Request.Context(), types.FindServiceParams{InstanceID: instanceID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
			return
		}
		if discovered == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": discoveredServiceMessage})
			return
		}
		apiKey := config.APIKey
		config = *discovered
		config.APIKey = apiKey
	}
	config.URL = strings.TrimRight(config.URL, "/")

	if !serviceURLsValid(config) {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidServiceURLMessage})
		return
	}

	log.Debug().
		Str("instance", instanceID).
		Str("url", core.RedactURL(config.URL)).
		Str("access_url", core.RedactURL(config.AccessURL)).
		Str("display_name", config.DisplayName).
		Bool("api_key_set", config.APIKey != "").
		Msg("Saving configuration")

	existing, err := h.db.FindServiceBy(c.Request.Context(), types.FindServiceParams{InstanceID: instanceID})
	if err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
		return
	}

	if config.APIKey == "" {
		config.APIKey = existing.APIKey
	}

	if err := h.db.UpdateService(c.Request.Context(), &config); err != nil {
		log.Error().Err(err).Str("instance", instanceID).Msg("Error saving configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	h.serviceSaved(c.Request.Context(), &config)
	log.Info().Str("instance", instanceID).Msg("Successfully saved configuration")
	c.JSON(http.StatusOK, serviceConfigResponse(config))
}

type createServiceRequest struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey"`
	AccessURL   string `json:"accessUrl"`
}

func (h *SettingsHandler) CreateSettings(c *gin.Context) {
	var req createServiceRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	serviceType, ok := catalog.Lookup(req.Type)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown service type: " + req.Type})
		return
	}

	config := models.ServiceConfiguration{
		DisplayName: req.DisplayName,
		URL:         strings.TrimRight(req.URL, "/"),
		APIKey:      req.APIKey,
		AccessURL:   req.AccessURL,
	}
	if !serviceURLsValid(config) {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidServiceURLMessage})
		return
	}

	if err := h.db.CreateServiceInstance(c.Request.Context(), serviceType.Name, &config); err != nil {
		log.Error().Err(err).Str("type", serviceType.Name).Msg("Error creating configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	h.serviceSaved(c.Request.Context(), &config)
	log.Info().Str("instance", config.InstanceID).Msg("Successfully created configuration")
	c.JSON(http.StatusCreated, serviceConfigResponse(config))
}

func (h *SettingsHandler) serviceSaved(ctx context.Context, config *models.ServiceConfiguration) {
	h.serviceManager.InitializeService(ctx, config)

	if err := InvalidateSettingsCache(ctx, h.cache); err != nil {
		log.Warn().Err(err).Msg("Failed to delete configuration cache")
	}

	if h.poller != nil {
		h.poller.Refresh(config.InstanceID)
	}
}

func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	instanceID := c.Param("instance")
	if models.AllowedEdits(instanceID) != models.EditsAll {
		c.JSON(http.StatusForbidden, gin.H{"error": discoveredServiceMessage})
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
