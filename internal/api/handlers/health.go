// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/catalog"
	"github.com/autobrr/dashbrr/internal/types"
)

// DatabaseService defines the database operations needed by HealthHandler
type DatabaseService interface {
	FindServiceBy(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error)
}

const serviceTestTimeout = 10 * time.Second

type HealthHandler struct {
	db             DatabaseService
	serviceCreator models.ServiceCreator
	timeout        time.Duration
}

func NewHealthHandler(db DatabaseService, creator ...models.ServiceCreator) *HealthHandler {
	var sc models.ServiceCreator
	if len(creator) > 0 {
		sc = creator[0]
	} else {
		sc = catalog.Registry{}
	}

	return &HealthHandler{
		db:             db,
		serviceCreator: sc,
		timeout:        serviceTestTimeout,
	}
}

func (h *HealthHandler) CheckHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	serviceID := c.Param("service")
	service, ok := h.storedService(ctx, c, serviceID)
	if !ok {
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}
	if service.URL == "" {
		c.JSON(http.StatusOK, models.ServiceHealth{
			Status:      "unconfigured",
			Message:     "Service is not configured",
			ServiceID:   serviceID,
			LastChecked: time.Now(),
		})
		return
	}

	h.checkService(ctx, c, serviceID, *service)
}

type serviceTestRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"apiKey"`
}

func (h *HealthHandler) TestService(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	var req serviceTestRequest
	if err := c.ShouldBindJSON(&req); err != nil || !isHTTPURL(req.URL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidServiceURLMessage})
		return
	}

	serviceID := c.Param("service")
	service := models.ServiceConfiguration{InstanceID: serviceID, URL: req.URL, APIKey: req.APIKey}
	if service.APIKey == "" {
		stored, ok := h.storedService(ctx, c, serviceID)
		if !ok {
			return
		}
		if stored != nil {
			service.APIKey = stored.APIKey
		}
	}

	h.checkService(ctx, c, serviceID, service)
}

func (h *HealthHandler) storedService(ctx context.Context, c *gin.Context, serviceID string) (*models.ServiceConfiguration, bool) {
	service, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: serviceID})
	if err == nil {
		return service, true
	}
	if ctx.Err() != nil {
		log.Error().Err(ctx.Err()).Str("service", serviceID).Msg("Context canceled while fetching service configuration")
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
		return nil, false
	}
	log.Error().Err(err).Str("service", serviceID).Msg("Failed to fetch service configuration")
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service configuration"})
	return nil, false
}

func (h *HealthHandler) checkService(ctx context.Context, c *gin.Context, serviceID string, service models.ServiceConfiguration) {
	serviceType, _, _ := strings.Cut(serviceID, "-")
	serviceChecker := h.serviceCreator.CreateService(serviceType)
	if serviceChecker == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported service type: " + serviceType})
		return
	}

	if !serviceHasCredentials(serviceType, service.URL, service.APIKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "API key is required for this service type",
		})
		return
	}

	health, statusCode := serviceChecker.CheckHealth(ctx, service.URL, service.APIKey)

	if ctx.Err() != nil {
		log.Error().Err(ctx.Err()).Str("service", serviceID).Msg("Context canceled during health check")
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("The service did not answer within %s", h.timeout),
		})
		return
	}

	if statusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"status":  "error",
			"message": serviceFailureMessage(statusCode, health.Message),
		})
		return
	}

	c.JSON(http.StatusOK, health)
}

func serviceFailureMessage(statusCode int, message string) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return "Invalid API key"
	case http.StatusNotFound:
		return "Service not found at the specified URL"
	}
	if message != "" {
		return message
	}
	if statusCode == http.StatusServiceUnavailable {
		return "Service is unavailable"
	}
	return "Failed to check service health"
}
