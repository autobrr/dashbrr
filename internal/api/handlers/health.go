// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/types"
)

// DatabaseService defines the database operations needed by HealthHandler
type DatabaseService interface {
	FindServiceBy(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error)
}

type HealthHandler struct {
	db             DatabaseService
	health         *services.HealthService
	serviceCreator models.ServiceCreator
}

func NewHealthHandler(db DatabaseService, health *services.HealthService, creator ...models.ServiceCreator) *HealthHandler {
	var sc models.ServiceCreator
	if len(creator) > 0 {
		sc = creator[0]
	} else {
		sc = models.NewServiceRegistry()
	}

	return &HealthHandler{
		db:             db,
		health:         health,
		serviceCreator: sc,
	}
}

func (h *HealthHandler) CheckHealth(c *gin.Context) {
	// Create a context with timeout for the entire health check operation
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	serviceID := c.Param("service")
	if serviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Service ID is required"})
		return
	}

	// Get URL and API key from query parameters for validation
	url := c.Query("url")
	apiKey := c.Query("apiKey")

	var service *models.ServiceConfiguration
	var err error

	if url != "" {
		service = &models.ServiceConfiguration{
			InstanceID: serviceID,
			URL:        url,
			APIKey:     apiKey,
		}
	} else {
		// Use context with timeout for database operation
		service, err = h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: serviceID})
		if err != nil {
			// Check for context cancellation
			if ctx.Err() != nil {
				log.Error().Err(ctx.Err()).Str("service", serviceID).Msg("Context canceled while fetching service configuration")
				c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
				return
			}
			log.Error().Err(err).Str("service", serviceID).Msg("Failed to fetch service configuration")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service configuration"})
			return
		}

		if service == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
			return
		}
	}

	// Check if service is configured
	if service.URL == "" {
		c.JSON(http.StatusOK, models.ServiceHealth{
			Status:      "unconfigured",
			Message:     "Service is not configured",
			ServiceID:   serviceID,
			LastChecked: time.Now(),
		})
		return
	}

	// Validate service ID format and extract service type
	parts := strings.Split(serviceID, "-")
	if len(parts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID format"})
		return
	}
	serviceType := parts[0]

	serviceChecker := h.serviceCreator.CreateService(serviceType)
	if serviceChecker == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported service type: " + serviceType})
		return
	}

	// For general service, API key is optional
	// For other services, ensure API key is provided
	if serviceType != "general" && service.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "API key is required for this service type",
		})
		return
	}

	// Use the context with timeout for health check
	health, statusCode := serviceChecker.CheckHealth(ctx, service.URL, service.APIKey)

	// Check for context cancellation after health check
	if ctx.Err() != nil {
		log.Error().Err(ctx.Err()).Str("service", serviceID).Msg("Context canceled during health check")
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"status":  "error",
			"message": "Health check timed out",
		})
		return
	}

	// Enhance error handling for specific status codes
	if statusCode != http.StatusOK {
		var errorMessage string
		switch statusCode {
		case http.StatusUnauthorized:
			errorMessage = "Invalid API key"
		case http.StatusNotFound:
			errorMessage = "Service not found at the specified URL"
		case http.StatusServiceUnavailable:
			errorMessage = "Service is unavailable"
		default:
			errorMessage = "Failed to check service health"
		}

		c.JSON(statusCode, gin.H{
			"status":  "error",
			"message": errorMessage,
		})
		return
	}

	c.JSON(http.StatusOK, health)
}
