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
	"github.com/autobrr/dashbrr/internal/utils"
)

const (
	overseerrStaleDataDuration = 5 * time.Minute
	overseerrCachePrefix       = "overseerr:requests:"
)

type OverseerrHandler struct {
	db             *database.DB
	cache          cache.Store
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastRequestsHash map[string]string
	hashMu           sync.Mutex
}

func NewOverseerrHandler(db *database.DB, cache cache.Store) *OverseerrHandler {
	return &OverseerrHandler{
		db:               db,
		cache:            cache,
		sf:               &singleflight.Group{},
		circuitBreaker:   resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastRequestsHash: make(map[string]string),
	}
}

func (h *OverseerrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (*types.RequestsStats, error)) (*types.RequestsStats, error) {
	var data types.RequestsStats

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.OverseerrRequests + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					_ = h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.OverseerrRequests)
				}
			}
		}()
		return &data, nil
	}

	// Check circuit breaker before making request
	if h.circuitBreaker.IsOpen() {
		// Try to get stale data when circuit is open
		var staleData types.RequestsStats
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return &staleData, nil
		}
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// Cache miss or error, fetch fresh data with retry
	var freshData *types.RequestsStats
	err = resilience.RetryWithBackoff(ctx, func() error {
		var fetchErr error
		freshData, fetchErr = fetchFn()
		return fetchErr
	})

	if err != nil {
		h.circuitBreaker.RecordFailure()
		// Try to get stale data
		var staleData types.RequestsStats
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return &staleData, nil
		}
		return nil, err
	}

	h.circuitBreaker.RecordSuccess()

	// Cache the fresh data
	if err := h.cache.Set(ctx, cacheKey, freshData, middleware.CacheDurations.OverseerrRequests); err == nil {
		// Also cache as stale data with longer duration
		_ = h.cache.Set(ctx, cacheKey+":stale", freshData, overseerrStaleDataDuration)
	}

	return freshData, nil
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
		approve = true
	} else if status == "3" {
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
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchRequests(instanceId)
	})

	if err == nil && result != nil {
		stats, err := utils.SafeConvert[*types.RequestsStats](result)
		if err == nil {
			h.broadcastOverseerrRequests(instanceId, stats)
		}
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
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchDataWithCache(ctx, cacheKey, func() (*types.RequestsStats, error) {
			return h.fetchRequests(instanceId)
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

	stats, err := utils.SafeConvert[*types.RequestsStats](result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
		return
	}

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

	c.JSON(http.StatusOK, stats)
}

func (h *OverseerrHandler) fetchRequests(instanceId string) (*types.RequestsStats, error) {
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
	if stats == nil {
		return
	}

	serviceStatus := "online"
	message := "overseerr_requests"

	// Set warning status if there are pending requests
	if stats.PendingCount > 0 {
		serviceStatus = "warning"
		message = fmt.Sprintf("%d pending requests", stats.PendingCount)
	}

	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      serviceStatus,
		Message:     message,
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"overseerr": types.OverseerrStats{
				Requests:     stats.Requests,
				PendingCount: stats.PendingCount,
			},
		},
		Details: map[string]interface{}{
			"overseerr": types.OverseerrDetails{
				PendingCount:  stats.PendingCount,
				TotalRequests: len(stats.Requests),
			},
		},
	}

	BroadcastHealth(health)
}

// createOverseerrRequestsHash generates a deterministic hash of the requests state
func createOverseerrRequestsHash(stats *types.RequestsStats) (string, []string) {
	if stats == nil || len(stats.Requests) == 0 {
		return "", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d:", stats.PendingCount)

	changes := make([]string, 0, len(stats.Requests))

	// Sort requests by ID for consistent hashing
	sortedRequests := make([]types.MediaRequest, len(stats.Requests))
	copy(sortedRequests, stats.Requests)
	sort.Slice(sortedRequests, func(i, j int) bool {
		return sortedRequests[i].ID < sortedRequests[j].ID
	})

	for _, req := range sortedRequests {
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

		// Create a deterministic hash string for each request
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
