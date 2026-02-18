// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	radarrQueuePrefix       = "radarr:queue:"
	radarrStaleDataDuration = 5 * time.Minute
)

type RadarrHandler struct {
	db              *database.DB
	cache           cache.Store
	bc              *Broadcaster
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewRadarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *RadarrHandler {
	return &RadarrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastQueueHash:  make(map[string]string),
	}
}

func (h *RadarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Radarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if !strings.HasPrefix(instanceId, "radarr") {
		log.Error().Str("instanceId", instanceId).Msg("[Radarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Radarr instance ID"})
		return
	}

	cacheKey := radarrQueuePrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.RadarrQueueResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.RadarrStatus,
		StaleTTL:       radarrStaleDataDuration,
		Fetch: func() (types.RadarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceId)
		},
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("[Radarr] Failed to fetch queue")

			if arrErr.HttpCode > 0 {
				c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	// Add hash-based change detection
	h.compareAndLogQueueChanges(instanceId, &result)

	if result.Records != nil {
		// Broadcast queue update via SSE
		h.broadcastRadarrQueue(instanceId, &result)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Radarr] Retrieved empty queue")
	}

	c.JSON(http.StatusOK, result)
}

func (h *RadarrHandler) fetchQueue(ctx context.Context, instanceId string) (types.RadarrQueueResponse, error) {
	radarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.RadarrQueueResponse{}, err
	}

	if radarrConfig == nil {
		return types.RadarrQueueResponse{}, NewServiceNotConfigured("radarr")
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Get queue records using the service
	records, err := service.GetQueueForHealth(ctx, radarrConfig.URL, radarrConfig.APIKey)
	if err != nil {
		return types.RadarrQueueResponse{}, err
	}

	// Create response
	return types.RadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}, nil
}

// compareAndLogQueueChanges tracks and logs changes in Radarr queue
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

	// Match frontend shape: stats.radarr.queue
	stats := map[string]interface{}{
		"radarr": map[string]interface{}{
			"queue": queueResp,
		},
	}

	details := map[string]interface{}{
		"radarr": types.RadarrQueueStats{
			TotalRecords:     queueResp.TotalRecords,
			DownloadingCount: downloading,
			TotalSize:        totalSize,
		},
	}

	// Use the existing BroadcastHealth function with a special message type
	h.bc.Publish(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "radarr_queue",
		LastChecked: time.Now(),
		Stats:       stats,
		Details:     details,
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

	ctx := c.Request.Context()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceId, queueId, options)
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Str("queueId", queueId).
				Msg("[Radarr] Failed to delete queue item")

			if arrErr.HttpCode > 0 {
				c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
		return
	}

	// Clear cache after successful deletion
	cacheKey := radarrQueuePrefix + instanceId
	if err := DeleteSWRCacheKeys(ctx, h.cache, cacheKey); err != nil {
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("[Radarr] Failed to clear cache after queue item deletion")
	}

	// Fetch fresh queue data
	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.RadarrQueueResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.RadarrStatus,
		StaleTTL:       radarrStaleDataDuration,
		Fetch: func() (types.RadarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceId)
		},
	})

	if err == nil {
		h.broadcastRadarrQueue(instanceId, &result)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *RadarrHandler) deleteQueueItem(ctx context.Context, instanceId, queueId string, options types.RadarrQueueDeleteOptions) error {
	radarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return err
	}

	if radarrConfig == nil {
		return NewServiceNotConfigured("radarr")
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(ctx, radarrConfig.URL, radarrConfig.APIKey, queueId, options)
}
