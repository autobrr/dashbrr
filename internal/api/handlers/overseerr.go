// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
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
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	overseerrStaleDataDuration = 5 * time.Minute
	overseerrCachePrefix       = "overseerr:requests:"
)

type OverseerrHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastRequestsHash map[string]string
	hashMu           sync.Mutex
}

func NewOverseerrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *OverseerrHandler {
	return &OverseerrHandler{
		db:               db,
		cache:            cache,
		bc:               bc,
		sf:               &singleflight.Group{},
		circuitBreaker:   resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastRequestsHash: make(map[string]string),
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
		approve = true
	} else if status == "3" {
		approve = false
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	ctx := c.Request.Context()

	overseerrConfig, err := findServiceConfig(ctx, h.db, instanceId)
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
	if err := DeleteSWRCacheKeys(ctx, h.cache, cacheKey); err != nil {
		log.Warn().Err(err).Str("instanceId", instanceId).Msg("Failed to clear cache after status update")
	}

	// Fetch fresh data and broadcast update using singleflight
	sfKey = fmt.Sprintf("requests:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchRequests(ctx, instanceId)
	})

	if err == nil && result != nil {
		stats := result.(*types.RequestsStats)
		if stats != nil && stats.Requests == nil {
			stats.Requests = []types.MediaRequest{}
		}
		h.broadcastOverseerrRequests(instanceId, stats)
	}

	c.Status(http.StatusOK)
}

func (h *OverseerrHandler) GetRequests(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "overseerr", "Overseerr")
	if !ok {
		return
	}

	cacheKey := overseerrCachePrefix + instanceId
	ctx := c.Request.Context()

	sfKey := fmt.Sprintf("requests:%s", instanceId)
	statsVal, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.RequestsStats]{
		Store:           h.cache,
		Key:             cacheKey,
		FreshTTL:        middleware.CacheDurations.OverseerrRequests,
		StaleTTL:        overseerrStaleDataDuration,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    h.sf,
		SingleflightKey: sfKey,
		Fetch: func() (types.RequestsStats, error) {
			fresh, err := h.fetchRequests(ctx, instanceId)
			if err != nil {
				return types.RequestsStats{}, err
			}
			if fresh == nil {
				return types.RequestsStats{
					PendingCount: 0,
					Requests:     []types.MediaRequest{},
				}, nil
			}
			if fresh.Requests == nil {
				fresh.Requests = []types.MediaRequest{}
			}
			return *fresh, nil
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
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

	stats := &statsVal

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

func (h *OverseerrHandler) fetchRequests(ctx context.Context, instanceId string) (*types.RequestsStats, error) {
	overseerrConfig, err := requireServiceConfigLegacy(ctx, h.db, instanceId)
	if err != nil {
		return nil, err
	}

	service := &overseerr.OverseerrService{}
	service.SetDB(h.db)

	stats, err := service.GetRequests(ctx, overseerrConfig.URL, overseerrConfig.APIKey)
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

	publishInternalServiceUpdate(h.bc, buildOverseerrRequestsServiceUpdate(instanceId, stats))
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
