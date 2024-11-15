// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	overseerrCachePrefix       = "overseerr:requests:"
	overseerrStaleDataDuration = 5 * time.Minute
)

type OverseerrHandler struct {
	db             *database.DB
	cache          cache.Store
	sf             singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastRequestsHash map[string]string
	hashMu           sync.Mutex
}

func NewOverseerrHandler(db *database.DB, cache cache.Store) *OverseerrHandler {
	return &OverseerrHandler{
		db:               db,
		cache:            cache,
		circuitBreaker:   resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastRequestsHash: make(map[string]string),
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *OverseerrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.OverseerrRequests + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.OverseerrRequests)
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
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.OverseerrRequests); err == nil {
		// Also cache as stale data with longer duration
		h.cache.Set(ctx, cacheKey+":stale", data, overseerrStaleDataDuration)
	}

	return data, nil
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

	// Update request status using singleflight with retry and circuit breaker
	sfKey := fmt.Sprintf("update_status:%s:%s", instanceId, requestId)
	ctx := context.Background()

	if h.circuitBreaker.IsOpen() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service is temporarily unavailable"})
		return
	}

	_, err, _ = h.sf.Do(sfKey, func() (interface{}, error) {
		return nil, resilience.RetryWithBackoff(ctx, func() error {
			return service.UpdateRequestStatus(ctx, overseerrConfig.URL, overseerrConfig.APIKey, reqID, approve)
		})
	})

	if err != nil {
		h.circuitBreaker.RecordFailure()
		log.Error().Err(err).
			Str("instanceId", instanceId).
			Int("requestId", reqID).
			Bool("approve", approve).
			Msg("Failed to update request status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update request status: %v", err)})
		return
	}

	h.circuitBreaker.RecordSuccess()

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

	// Use singleflight to prevent duplicate requests
	sfKey := fmt.Sprintf("requests:%s", instanceId)
	statsI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
			return h.fetchAndCacheRequests(instanceId, cacheKey)
		})
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
		h.hashMu.Lock()
		currentHash, changes := createOverseerrRequestsHash(stats)
		lastHash := h.lastRequestsHash[instanceId]

		// Only log and update if there are requests and the hash has changed
		if len(stats.Requests) > 0 && (lastHash == "" || currentHash != lastHash) {
			log.Debug().
				Str("instanceId", instanceId).
				Int("size", len(stats.Requests)).
				Msg("[Overseerr] Successfully retrieved and cached requests")

			// Log changes if hash is different
			if lastHash != "" && currentHash != lastHash {
				log.Debug().
					Str("instanceId", instanceId).
					Strs("changes", changes).
					Msg("Overseerr requests hash changed")
			}

			// Update the last hash
			h.lastRequestsHash[instanceId] = currentHash
		}
		h.hashMu.Unlock()

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

	return stats, nil
}

func (h *OverseerrHandler) broadcastOverseerrRequests(instanceId string, stats *types.RequestsStats) {
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

func createOverseerrRequestsHash(stats *types.RequestsStats) (string, []string) {
	if stats == nil || len(stats.Requests) == 0 {
		return "", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d:", stats.PendingCount)

	changes := []string{}

	sort.Slice(stats.Requests, func(i, j int) bool {
		return stats.Requests[i].ID < stats.Requests[j].ID
	})

	for _, req := range stats.Requests {
		reqDetails := fmt.Sprintf("Full Request Details: "+
			"ID=%d, Status=%d, MediaType=%s, MediaTitle=%s, "+
			"RequestedBy.ID=%d, RequestedBy.Username=%s, "+
			"RequestedBy.Email=%s, RequestedBy.PlexUsername=%s",
			req.ID,
			req.Status,
			req.Media.MediaType,
			req.Media.Title,
			req.RequestedBy.ID,
			req.RequestedBy.Username,
			req.RequestedBy.Email,
			req.RequestedBy.PlexUsername)

		reqHash := fmt.Sprintf("%d:%d:%s:%s:%s:%s",
			req.ID,
			req.Status,
			req.Media.MediaType,
			req.RequestedBy.Username,
			req.Media.Title,
			reqDetails)

		sb.WriteString(reqHash + ",")

		changes = append(changes, reqDetails)
	}

	return sb.String(), changes
}
