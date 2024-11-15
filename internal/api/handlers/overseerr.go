// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/types"
)

const overseerrCachePrefix = "overseerr:requests:"

type OverseerrHandler struct {
	db    *database.DB
	cache cache.Store
	sf    singleflight.Group
}

func NewOverseerrHandler(db *database.DB, cache cache.Store) *OverseerrHandler {
	return &OverseerrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *OverseerrHandler) UpdateRequestStatus(c *gin.Context) {
	instanceId := c.Param("instanceId")
	requestId := c.Param("requestId")
	status := c.Param("status")

	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if requestId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requestId is required"})
		return
	}

	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	// Convert request ID to integer
	reqID, err := strconv.Atoi(requestId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	// Convert numeric status to approve/decline
	approve := false
	if status == "2" {
		status = "approve"
		approve = true
	} else if status == "3" {
		status = "decline"
		approve = false
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	// Get service configuration
	overseerrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get service configuration")
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	if overseerrConfig == nil || overseerrConfig.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Service not configured"})
		return
	}

	// Create Overseerr service instance
	service := &overseerr.OverseerrService{}
	service.SetDB(h.db)

	// Update request status using singleflight
	sfKey := fmt.Sprintf("update_status:%s:%s", instanceId, requestId)
	_, err, _ = h.sf.Do(sfKey, func() (interface{}, error) {
		return nil, service.UpdateRequestStatus(context.Background(), overseerrConfig.URL, overseerrConfig.APIKey, reqID, approve)
	})

	if err != nil {
		log.Error().Err(err).
			Str("instanceId", instanceId).
			Int("requestId", reqID).
			Bool("approve", approve).
			Msg("Failed to update request status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update request status: %v", err)})
		return
	}

	// Clear the cache for this instance to force a refresh
	cacheKey := overseerrCachePrefix + instanceId
	if err := h.cache.Delete(context.Background(), cacheKey); err != nil {
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("Failed to clear cache after status update")
	}

	// Fetch fresh data and broadcast update using singleflight
	sfKey = fmt.Sprintf("requests:%s", instanceId)
	statsI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheRequests(instanceId, cacheKey)
	})

	if err == nil && statsI != nil {
		stats := statsI.(*types.RequestsStats)
		h.broadcastOverseerrRequests(instanceId, stats)
	}

	c.Status(http.StatusOK)
}

func (h *OverseerrHandler) GetRequests(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is an Overseerr instance
	if instanceId[:9] != "overseerr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Overseerr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Overseerr instance ID"})
		return
	}

	cacheKey := overseerrCachePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var response *types.RequestsStats
	err := h.cache.Get(ctx, cacheKey, &response)
	if err == nil && response != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", len(response.Requests)).
			Msg("Serving Overseerr requests from cache")
		c.JSON(http.StatusOK, response)

		// Refresh cache in background using singleflight
		go func() {
			refreshKey := fmt.Sprintf("requests_refresh:%s", instanceId)
			_, _, _ = h.sf.Do(refreshKey, func() (interface{}, error) {
				h.refreshRequestsCache(instanceId, cacheKey)
				return nil, nil
			})
		}()
		return
	}

	// If not in cache, fetch from service using singleflight
	sfKey := fmt.Sprintf("requests:%s", instanceId)
	statsI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheRequests(instanceId, cacheKey)
	})

	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			c.JSON(http.StatusOK, &types.RequestsStats{
				PendingCount: 0,
				Requests:     []types.MediaRequest{},
			})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Overseerr requests")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Overseerr requests")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	stats := statsI.(*types.RequestsStats)

	if stats != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", len(stats.Requests)).
			Msg("Successfully retrieved and cached Overseerr requests")

		// Broadcast the fresh data
		h.broadcastOverseerrRequests(instanceId, stats)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Retrieved empty Overseerr requests")
	}

	c.JSON(http.StatusOK, stats)
}

func (h *OverseerrHandler) fetchAndCacheRequests(instanceId, cacheKey string) (*types.RequestsStats, error) {
	overseerrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, err
	}

	if overseerrConfig == nil || overseerrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &overseerr.OverseerrService{}
	service.SetDB(h.db)

	stats, err := service.GetRequests(context.Background(), overseerrConfig.URL, overseerrConfig.APIKey)
	if err != nil {
		return nil, err
	}

	if stats == nil {
		return nil, nil
	}

	// Initialize empty requests if nil
	if stats.Requests == nil {
		stats.Requests = []types.MediaRequest{}
	}

	// Cache the results using the centralized cache duration
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, stats, middleware.CacheDurations.OverseerrRequests); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Overseerr requests")
	}

	return stats, nil
}

func (h *OverseerrHandler) refreshRequestsCache(instanceId, cacheKey string) {
	stats, err := h.fetchAndCacheRequests(instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Overseerr requests cache")
		return
	}

	if stats != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", len(stats.Requests)).
			Msg("Successfully refreshed Overseerr requests cache")

		// Broadcast the updated data
		h.broadcastOverseerrRequests(instanceId, stats)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Refreshed cache with empty Overseerr requests")
	}
}

// broadcastOverseerrRequests broadcasts Overseerr request updates to all connected SSE clients
func (h *OverseerrHandler) broadcastOverseerrRequests(instanceId string, stats *types.RequestsStats) {
	// Use the existing BroadcastHealth function with a special message type
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "ok",
		Message:     "overseerr_requests",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"overseerr": stats,
		},
		Details: map[string]interface{}{
			"overseerr": map[string]interface{}{
				"pendingCount":  stats.PendingCount,
				"totalRequests": len(stats.Requests),
			},
		},
	})
}
