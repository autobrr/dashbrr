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

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/services/omegabrr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	omegabrrCacheDuration = 30 * time.Second
	omegabrrStatusPrefix  = "omegabrr:status:"
)

type OmegabrrHandler struct {
	db    *database.DB
	cache cache.Store
	sf    singleflight.Group
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
		db:    db,
		cache: cache,
	}
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

	// Try to get from cache first
	var health models.ServiceHealth
	err := h.cache.Get(ctx, cacheKey, &health)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Serving Omegabrr status from cache")
		c.JSON(http.StatusOK, health)

		// Refresh cache in background using singleflight
		go func() {
			refreshKey := fmt.Sprintf("status_refresh:%s", instanceId)
			_, _, _ = h.sf.Do(refreshKey, func() (interface{}, error) {
				h.refreshStatusCache(instanceId, cacheKey)
				return nil, nil
			})
		}()
		return
	}

	// If not in cache, fetch from service using singleflight
	sfKey := fmt.Sprintf("status:%s", instanceId)
	healthI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheStatus(ctx, instanceId, cacheKey)
	})

	if err != nil {
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

	health = healthI.(models.ServiceHealth)

	log.Info().
		Str("instanceId", instanceId).
		Msg("[Omegabrr] Successfully retrieved and cached status")

	c.JSON(http.StatusOK, health)
}

func (h *OmegabrrHandler) fetchAndCacheStatus(ctx context.Context, instanceId, cacheKey string) (models.ServiceHealth, error) {
	omegabrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return models.ServiceHealth{}, err
	}

	if omegabrrConfig == nil {
		return models.ServiceHealth{}, fmt.Errorf("omegabrr is not configured")
	}

	service := &omegabrr.OmegabrrService{
		ServiceCore: core.ServiceCore{},
	}

	health, statusCode := service.CheckHealth(ctx, omegabrrConfig.URL, omegabrrConfig.APIKey)
	if statusCode != http.StatusOK {
		return models.ServiceHealth{}, fmt.Errorf("failed to get status")
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, health, omegabrrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Omegabrr status")
	}

	return health, nil
}

func (h *OmegabrrHandler) refreshStatusCache(instanceId, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	_, err := h.fetchAndCacheStatus(ctx, instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Omegabrr status cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully refreshed Omegabrr status cache")
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

	if req.APIKey == "" || req.TargetURL == "" {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "API key and target URL are required",
		})
		return
	}

	// Use singleflight to prevent duplicate webhook triggers
	sfKey := fmt.Sprintf("webhook_arrs:%s", req.TargetURL)
	resultI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		service := &omegabrr.OmegabrrService{
			ServiceCore: core.ServiceCore{},
		}
		return service.TriggerARRsWebhook(c, req.TargetURL, req.APIKey), nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to execute webhook")
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Failed to trigger ARRs webhook",
		})
		return
	}

	statusCode := resultI.(int)
	if statusCode != http.StatusOK {
		log.Error().
			Str("targetUrl", req.TargetURL).
			Int("statusCode", statusCode).
			Msg("Failed to trigger ARRs webhook")
		c.JSON(statusCode, WebhookResponse{
			Success: false,
			Message: "Failed to trigger ARRs webhook",
		})
		return
	}

	log.Info().
		Str("targetUrl", req.TargetURL).
		Msg("Successfully triggered ARRs webhook")

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "ARRs webhook triggered successfully",
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

	if req.APIKey == "" || req.TargetURL == "" {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "API key and target URL are required",
		})
		return
	}

	// Use singleflight to prevent duplicate webhook triggers
	sfKey := fmt.Sprintf("webhook_lists:%s", req.TargetURL)
	resultI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		service := &omegabrr.OmegabrrService{
			ServiceCore: core.ServiceCore{},
		}
		return service.TriggerListsWebhook(c, req.TargetURL, req.APIKey), nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to execute webhook")
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Failed to trigger Lists webhook",
		})
		return
	}

	statusCode := resultI.(int)
	if statusCode != http.StatusOK {
		log.Error().
			Str("targetUrl", req.TargetURL).
			Int("statusCode", statusCode).
			Msg("Failed to trigger Lists webhook")
		c.JSON(statusCode, WebhookResponse{
			Success: false,
			Message: "Failed to trigger Lists webhook",
		})
		return
	}

	log.Info().
		Str("targetUrl", req.TargetURL).
		Msg("Successfully triggered Lists webhook")

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Lists webhook triggered successfully",
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

	if req.APIKey == "" || req.TargetURL == "" {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "API key and target URL are required",
		})
		return
	}

	// Use singleflight to prevent duplicate webhook triggers
	sfKey := fmt.Sprintf("webhook_all:%s", req.TargetURL)
	resultI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		service := &omegabrr.OmegabrrService{
			ServiceCore: core.ServiceCore{},
		}
		return service.TriggerAllWebhooks(c, req.TargetURL, req.APIKey), nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to execute webhook")
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Failed to trigger all webhooks",
		})
		return
	}

	statusCode := resultI.(int)
	if statusCode != http.StatusOK {
		log.Error().
			Str("targetUrl", req.TargetURL).
			Int("statusCode", statusCode).
			Msg("Failed to trigger all webhooks")
		c.JSON(statusCode, WebhookResponse{
			Success: false,
			Message: "Failed to trigger all webhooks",
		})
		return
	}

	log.Info().
		Str("targetUrl", req.TargetURL).
		Msg("Successfully triggered all webhooks")

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "All webhooks triggered successfully",
	})
}
