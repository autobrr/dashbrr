package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/models"
	"github.com/autobrr/dashbrr/backend/services"
)

// DatabaseService defines the database operations needed by HealthHandler
type DatabaseService interface {
	GetServiceByInstanceID(id string) (*models.ServiceConfiguration, error)
}

type HealthHandler struct {
	db             DatabaseService
	health         *services.HealthService
	serviceFactory models.ServiceFactory
}

func NewHealthHandler(db DatabaseService, health *services.HealthService, factory ...models.ServiceFactory) *HealthHandler {
	var sf models.ServiceFactory
	if len(factory) > 0 {
		sf = factory[0]
	} else {
		sf = models.NewServiceFactory()
	}

	return &HealthHandler{
		db:             db,
		health:         health,
		serviceFactory: sf,
	}
}

func (h *HealthHandler) CheckHealth(c *gin.Context) {
	serviceID := c.Param("service")
	if serviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Service ID is required"})
		return
	}

	// Get URL and API key from query parameters for validation
	url := c.Query("url")
	apiKey := c.Query("apiKey")

	// If URL or API key is provided in query params, use those for validation
	// Otherwise, fetch from database
	var service *models.ServiceConfiguration
	var err error

	if url != "" && apiKey != "" {
		service = &models.ServiceConfiguration{
			InstanceID: serviceID,
			URL:        url,
			APIKey:     apiKey,
		}
	} else {
		service, err = h.db.GetServiceByInstanceID(serviceID)
		if err != nil {
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

	serviceChecker := h.serviceFactory.CreateService(serviceType)
	if serviceChecker == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported service type: " + serviceType})
		return
	}

	health, statusCode := serviceChecker.CheckHealth(service.URL, service.APIKey)

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
