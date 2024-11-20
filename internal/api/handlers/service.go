package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type ServiceHandler struct {
	db             *database.DB
	health         *services.HealthService
	cache          cache.Store
	serviceManager *services.ServiceManager
	lastDebugLog   time.Time
}

func NewServiceHandler(db *database.DB, cache cache.Store, serviceManager *services.ServiceManager, health *services.HealthService) *ServiceHandler {
	return &ServiceHandler{
		db:             db,
		health:         health,
		cache:          cache,
		serviceManager: serviceManager,
		lastDebugLog:   time.Now().Add(-configDebugLogTTL), // Initialize to ensure first log happens
	}
}

func (h *ServiceHandler) Create(c *gin.Context) {
	//instanceID := c.Param("instance")

	var config types.ServiceConfiguration
	if err := c.BindJSON(&config); err != nil {
		//log.Error().Err(err).Str("instance", instanceID).Msg("Error binding JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	//config.InstanceID = instanceID
	config.URL = strings.TrimRight(config.URL, "/")

	log.Debug().
		//Str("instance", instanceID).
		Interface("config", config).
		Msg("Saving configuration")

	// Check if configuration exists
	//existing, err := h.db.FindServiceBy(c.Request.Context(), types.FindServiceParams{InstanceID: instanceID})
	//if err != nil {
	//	if !errors.Is(err, sql.ErrNoRows) {
	//		log.Error().Err(err).Str("instance", instanceID).Msg("Error checking existing configuration")
	//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing configuration"})
	//		return
	//	}
	//}

	//// If updating, stop health monitoring first
	//if existing != nil && h.health != nil {
	//	h.health.StopMonitoring(instanceID)
	//}

	err := h.db.CreateService(c.Request.Context(), &config)
	if err != nil {
		log.Error().Err(err).Str("instance", config.InstanceID).Msg("Error saving configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	//var saveErr error
	//if existing == nil {
	//	// Create new configuration
	//	log.Debug().Str("instance", instanceID).Msg("Creating new configuration")
	//	saveErr = h.db.CreateService(c.Request.Context(), &config)
	//} else {
	//	// Update existing configuration
	//	log.Debug().Str("instance", instanceID).Msg("Updating existing configuration")
	//	saveErr = h.db.UpdateService(c.Request.Context(), &config)
	//}
	//
	//if saveErr != nil {
	//	log.Error().Err(saveErr).Str("instance", instanceID).Msg("Error saving configuration")
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
	//	return
	//}

	// Initialize service data
	if err := h.serviceManager.InitializeService(c.Request.Context(), &config); err != nil {
		log.Error().Err(err).Str("instance", config.InstanceID).Msg("Error initializing service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize service"})
	}

	// Invalidate cache
	//h.cache.Delete(c.Request.Context(), configCacheKey)

	log.Info().Str("instance", config.InstanceID).Msg("Successfully saved configuration")

	c.JSON(http.StatusOK, config)
}
