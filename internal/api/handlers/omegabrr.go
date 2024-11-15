// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/services/omegabrr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	omegabrrStaleDataDuration = 5 * time.Minute
	omegabrrStatusPrefix      = "omegabrr:status:"
)

type OmegabrrHandler struct {
	db             *database.DB
	cache          cache.Store
	sf             singleflight.Group
	circuitBreaker *resilience.CircuitBreaker
}

type WebhookRequest struct {
	TargetURL string `json:"targetUrl"`
	APIKey    string `json:"apiKey"`
}

type WebhookResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewOmegabrrHandler(db *database.DB, cache cache.Store) *OmegabrrHandler {
	return &OmegabrrHandler{
		db:             db,
		cache:          cache,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *OmegabrrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.OmegabrrStatus + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.OmegabrrStatus)
				}
			}
		}()
		return data, nil
	}

	// Check circuit breaker before making request
	if h.circuitBreaker.IsOpen() {
		// Try to get stale data when circuit is open
		var staleData interface{}
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// Cache miss or error, fetch fresh data with retry
	var fetchErr error
	err = resilience.RetryWithBackoff(ctx, func() error {
		data, fetchErr = fetchFn()
		return fetchErr
	})

	if err != nil {
		h.circuitBreaker.RecordFailure()
		// Try to get stale data
		var staleData interface{}
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return nil, err
	}

	h.circuitBreaker.RecordSuccess()

	// Cache the fresh data
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.OmegabrrStatus); err == nil {
		// Also cache as stale data with longer duration
		h.cache.Set(ctx, cacheKey+":stale", data, omegabrrStaleDataDuration)
	}

	return data, nil
}

func (h *OmegabrrHandler) GetOmegabrrStatus(c *gin.Context) {
	log.Debug().Msg("Starting to fetch Omegabrr status")

	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	cacheKey := omegabrrStatusPrefix + instanceId
	ctx := context.Background()

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("status:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
			return h.fetchStatus(ctx, instanceId)
		})
	})

	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, models.ServiceHealth{})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Omegabrr status")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Omegabrr status")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	health := result.(models.ServiceHealth)
	c.JSON(http.StatusOK, health)
}

func (h *OmegabrrHandler) fetchStatus(ctx context.Context, instanceId string) (models.ServiceHealth, error) {
	omegabrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return models.ServiceHealth{}, err
	}

	if omegabrrConfig == nil {
		return models.ServiceHealth{}, fmt.Errorf("service not configured")
	}

	service := &omegabrr.OmegabrrService{
		ServiceCore: core.ServiceCore{},
	}

	health, statusCode := service.CheckHealth(ctx, omegabrrConfig.URL, omegabrrConfig.APIKey)
	if statusCode != http.StatusOK {
		return models.ServiceHealth{}, fmt.Errorf("failed to get status")
	}

	return health, nil
}

// executeWebhook handles webhook execution with resilience patterns
func (h *OmegabrrHandler) executeWebhook(c *gin.Context, webhookType string, req WebhookRequest, triggerFn func() int) {
	if req.APIKey == "" || req.TargetURL == "" {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "API key and target URL are required",
		})
		return
	}

	// Check circuit breaker before making request
	if h.circuitBreaker.IsOpen() {
		c.JSON(http.StatusServiceUnavailable, WebhookResponse{
			Success: false,
			Message: "Service is temporarily unavailable",
		})
		return
	}

	// Use singleflight to prevent duplicate webhook triggers
	sfKey := fmt.Sprintf("webhook_%s:%s", webhookType, req.TargetURL)
	var statusCode int
	_, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		var execErr error
		err := resilience.RetryWithBackoff(c, func() error {
			statusCode = triggerFn()
			if statusCode != http.StatusOK {
				execErr = fmt.Errorf("webhook returned status %d", statusCode)
				return execErr
			}
			return nil
		})
		if err != nil {
			return nil, execErr
		}
		return nil, nil
	})

	if err != nil {
		h.circuitBreaker.RecordFailure()
		log.Error().
			Err(err).
			Str("webhookType", webhookType).
			Str("targetUrl", req.TargetURL).
			Int("statusCode", statusCode).
			Msg("Failed to trigger webhook")

		c.JSON(statusCode, WebhookResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to trigger %s webhook", webhookType),
		})
		return
	}

	h.circuitBreaker.RecordSuccess()
	log.Info().
		Str("webhookType", webhookType).
		Str("targetUrl", req.TargetURL).
		Msg("Successfully triggered webhook")

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: fmt.Sprintf("%s webhook triggered successfully", webhookType),
	})
}

// TriggerWebhookArrs handles webhook trigger for ARRs
func (h *OmegabrrHandler) TriggerWebhookArrs(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to parse webhook request")
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	service := &omegabrr.OmegabrrService{
		ServiceCore: core.ServiceCore{},
	}
	h.executeWebhook(c, "ARRs", req, func() int {
		return service.TriggerARRsWebhook(c, req.TargetURL, req.APIKey)
	})
}

// TriggerWebhookLists handles webhook trigger for Lists
func (h *OmegabrrHandler) TriggerWebhookLists(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to parse webhook request")
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	service := &omegabrr.OmegabrrService{
		ServiceCore: core.ServiceCore{},
	}
	h.executeWebhook(c, "Lists", req, func() int {
		return service.TriggerListsWebhook(c, req.TargetURL, req.APIKey)
	})
}

// TriggerWebhookAll handles webhook trigger for all updates
func (h *OmegabrrHandler) TriggerWebhookAll(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to parse webhook request")
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	service := &omegabrr.OmegabrrService{
		ServiceCore: core.ServiceCore{},
	}
	h.executeWebhook(c, "All", req, func() int {
		return service.TriggerAllWebhooks(c, req.TargetURL, req.APIKey)
	})
}
