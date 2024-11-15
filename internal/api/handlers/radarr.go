// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

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
	db              *database.DB
	cache           cache.Store
	sf              singleflight.Group
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewRadarrHandler(db *database.DB, cache cache.Store) *RadarrHandler {
	return &RadarrHandler{
		db:            db,
		cache:         cache,
		lastQueueHash: make(map[string]string),
	}
}

// compareAndLogQueueChanges tracks and logs changes in Radarr queue
// It compares the current queue state with the previous state for a specific Radarr instance
// Helps detect queue changes like new downloads starting, downloads completing, or status updates
func (h *RadarrHandler) compareAndLogQueueChanges(instanceId string, queueResp *types.RadarrQueueResponse) {
	h.lastQueueHashMu.Lock()
	defer h.lastQueueHashMu.Unlock()

	wrapped := wrapRadarrQueue(queueResp)
	currentHash := generateQueueHash(wrapped)
	lastHash := h.lastQueueHash[instanceId]

	if currentHash != lastHash {
		changes := detectQueueChanges(lastHash, currentHash)
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Str("change", changes).
			Msg("[Radarr] Queue changed")

		h.lastQueueHash[instanceId] = currentHash
	}
}

func (h *RadarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Radarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Radarr instance
	if instanceId[:6] != "radarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Radarr] Invalid instance ID")
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
			Msg("[Radarr] Serving queue from cache")
		c.JSON(http.StatusOK, queueResp)

		// Refresh cache in background using singleflight
		go func() {
			refreshKey := fmt.Sprintf("queue_refresh:%s", instanceId)
			_, _, _ = h.sf.Do(refreshKey, func() (interface{}, error) {
				h.refreshQueueCache(instanceId, cacheKey)
				return nil, nil
			})
		}()
		return
	}

	// If not in cache, fetch from service using singleflight
	sfKey := fmt.Sprintf("queue:%s", instanceId)
	queueRespI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheQueue(instanceId, cacheKey)
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("[Radarr] Failed to fetch queue")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	queueResp = queueRespI.(types.RadarrQueueResponse)

	// Add hash-based change detection
	h.compareAndLogQueueChanges(instanceId, &queueResp)

	if queueResp.Records != nil {
		// Broadcast queue update via SSE
		h.broadcastRadarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Radarr] Retrieved empty queue")
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
			Msg("[Radarr] Failed to cache queue")
	}

	return queueResp, nil
}

func (h *RadarrHandler) refreshQueueCache(instanceId, cacheKey string) {
	queueResp, err := h.fetchAndCacheQueue(instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Radarr] Failed to refresh queue cache")
		return
	}

	// Add hash-based change detection
	h.compareAndLogQueueChanges(instanceId, &queueResp)

	if queueResp.Records != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Radarr] Queue cache refreshed")

		// Broadcast queue update via SSE
		h.broadcastRadarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Radarr] Refreshed cache with empty queue")
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
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Radarr] Failed to get configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Radarr configuration"})
		return
	}

	if radarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("[Radarr] is not configured")
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
				Msg("[Radarr] Failed to delete queue item")

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
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("[Radarr] Failed to clear cache after queue item deletion")
	}

	// Fetch fresh data and broadcast update using singleflight
	sfKey := fmt.Sprintf("queue:%s", instanceId)
	queueRespI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheQueue(instanceId, cacheKey)
	})

	if err == nil {
		queueResp := queueRespI.(types.RadarrQueueResponse)
		h.broadcastRadarrQueue(instanceId, &queueResp)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}
