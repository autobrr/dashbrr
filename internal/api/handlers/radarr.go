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

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/types"
)

const radarrQueuePrefix = "radarr:queue:"

type RadarrHandler struct {
	db    *database.DB
	cache cache.Store
}

func NewRadarrHandler(db *database.DB, cache cache.Store) *RadarrHandler {
	return &RadarrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *RadarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Radarr instance
	if instanceId[:6] != "radarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Radarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Radarr instance ID"})
		return
	}

	cacheKey := radarrQueuePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var queueResp types.RadarrQueueResponse
	err := h.cache.Get(ctx, cacheKey, &queueResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Serving Radarr queue from cache")
		c.JSON(http.StatusOK, queueResp)

		// Refresh cache in background without delay
		go h.refreshQueueCache(instanceId, cacheKey)
		return
	}

	// If not in cache, fetch from service
	queueResp, err = h.fetchAndCacheQueue(instanceId, cacheKey)
	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("Failed to fetch Radarr queue")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	if queueResp.Records != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Successfully retrieved and cached Radarr queue")

		// Broadcast queue update via SSE
		h.broadcastRadarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Retrieved empty Radarr queue")
	}

	c.JSON(http.StatusOK, queueResp)
}

func (h *RadarrHandler) fetchAndCacheQueue(instanceId, cacheKey string) (types.RadarrQueueResponse, error) {
	radarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.RadarrQueueResponse{}, err
	}

	if radarrConfig == nil {
		return types.RadarrQueueResponse{}, fmt.Errorf("radarr is not configured")
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Get queue records using the service
	records, err := service.GetQueueForHealth(context.Background(), radarrConfig.URL, radarrConfig.APIKey)
	if err != nil {
		return types.RadarrQueueResponse{}, err
	}

	// Create response
	queueResp := types.RadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	// Cache the results using the centralized cache duration
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, queueResp, middleware.CacheDurations.RadarrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Radarr queue")
	}

	return queueResp, nil
}

func (h *RadarrHandler) refreshQueueCache(instanceId, cacheKey string) {
	queueResp, err := h.fetchAndCacheQueue(instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Radarr queue cache")
		return
	}

	if queueResp.Records != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Successfully refreshed Radarr queue cache")

		// Broadcast queue update via SSE
		h.broadcastRadarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Refreshed cache with empty Radarr queue")
	}
}

// broadcastRadarrQueue broadcasts Radarr queue updates to all connected SSE clients
func (h *RadarrHandler) broadcastRadarrQueue(instanceId string, queueResp *types.RadarrQueueResponse) {
	// Calculate additional statistics
	var totalSize int64
	var downloading int
	for _, record := range queueResp.Records {
		totalSize += record.Size
		if record.Status == "downloading" {
			downloading++
		}
	}

	// Use the existing BroadcastHealth function with a special message type
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "ok",
		Message:     "radarr_queue",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"radarr": queueResp,
		},
		Details: map[string]interface{}{
			"radarr": map[string]interface{}{
				"totalRecords":     queueResp.TotalRecords,
				"downloadingCount": downloading,
				"totalSize":        totalSize,
			},
		},
	})
}

// DeleteQueueItem handles the deletion of a queue item with specified options
func (h *RadarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	// Get options from query parameters
	options := types.RadarrQueueDeleteOptions{
		RemoveFromClient: c.Query("removeFromClient") == "true",
		Blocklist:        c.Query("blocklist") == "true",
		SkipRedownload:   c.Query("skipRedownload") == "true",
		ChangeCategory:   c.Query("changeCategory") == "true",
	}

	radarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Radarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Radarr configuration"})
		return
	}

	if radarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Radarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Radarr is not configured"})
		return
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Call the service method to delete the queue item
	if err := service.DeleteQueueItem(context.Background(), radarrConfig.URL, radarrConfig.APIKey, queueId, options); err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Str("queueId", queueId).
				Msg("Failed to delete queue item")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
		return
	}

	// Clear cache after successful deletion
	cacheKey := radarrQueuePrefix + instanceId
	if err := h.cache.Delete(context.Background(), cacheKey); err != nil {
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("Failed to clear cache after queue item deletion")
	}

	// Fetch fresh data and broadcast update
	queueResp, err := h.fetchAndCacheQueue(instanceId, cacheKey)
	if err == nil {
		h.broadcastRadarrQueue(instanceId, &queueResp)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}
