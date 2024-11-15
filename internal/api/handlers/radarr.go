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
	sf              *singleflight.Group
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewRadarrHandler(db *database.DB, cache cache.Store) *RadarrHandler {
	return &RadarrHandler{
		db:             db,
		cache:          cache,
		sf:             &singleflight.Group{},
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastQueueHash:  make(map[string]string),
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *RadarrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.RadarrStatus + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.RadarrStatus)
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
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.RadarrStatus); err == nil {
		// Also cache as stale data with longer duration
		h.cache.Set(ctx, cacheKey+":stale", data, radarrStaleDataDuration)
	}

	return data, nil
}

func (h *RadarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Radarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if instanceId[:6] != "radarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Radarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Radarr instance ID"})
		return
	}

	cacheKey := radarrQueuePrefix + instanceId
	ctx := context.Background()

	result, err := h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
		return h.fetchQueue(instanceId)
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

	queueResp := result.(types.RadarrQueueResponse)

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

func (h *RadarrHandler) fetchQueue(instanceId string) (types.RadarrQueueResponse, error) {
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

	ctx := context.Background()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(instanceId, queueId, options)
	})

	if err != nil {
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
	if err := h.cache.Delete(ctx, cacheKey); err != nil {
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("[Radarr] Failed to clear cache after queue item deletion")
	}

	// Fetch fresh queue data
	result, err := h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
		return h.fetchQueue(instanceId)
	})

	if err == nil {
		queueResp := result.(types.RadarrQueueResponse)
		h.broadcastRadarrQueue(instanceId, &queueResp)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *RadarrHandler) deleteQueueItem(instanceId, queueId string, options types.RadarrQueueDeleteOptions) error {
	radarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return err
	}

	if radarrConfig == nil {
		return fmt.Errorf("radarr is not configured")
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(context.Background(), radarrConfig.URL, radarrConfig.APIKey, queueId, options)
}
