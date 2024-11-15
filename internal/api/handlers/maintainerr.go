// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	maintainerrCachePrefix       = "maintainerr:collections:"
	maintainerrStaleDataDuration = 5 * time.Minute
	healthCheckTimeout           = 5 * time.Second // Reduced from 30s to 5s to match other handlers
)

type MaintainerrHandler struct {
	db             *database.DB
	cache          cache.Store
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastCollectionsHash   map[string]string
	lastCollectionsHashMu sync.Mutex
}

func NewMaintainerrHandler(db *database.DB, cache cache.Store) *MaintainerrHandler {
	return &MaintainerrHandler{
		db:                  db,
		cache:               cache,
		sf:                  &singleflight.Group{},
		circuitBreaker:      resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastCollectionsHash: make(map[string]string),
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *MaintainerrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.MaintainerrStatus + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.MaintainerrStatus)
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
			log.Warn().Msg("[Maintainerr] Circuit breaker open, serving stale data")
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
			log.Warn().Err(err).Msg("[Maintainerr] Failed to fetch fresh data, serving stale")
			return staleData, nil
		}
		return nil, err
	}

	h.circuitBreaker.RecordSuccess()

	// Cache the fresh data
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.MaintainerrStatus); err == nil {
		// Also cache as stale data with longer duration
		h.cache.Set(ctx, cacheKey+":stale", data, maintainerrStaleDataDuration)
	}

	return data, nil
}

// handleHTTPStatusCode processes HTTP status codes from Maintainerr errors
func handleHTTPStatusCode(code int) (int, string) {
	switch code {
	case http.StatusBadGateway:
		return code, "Service is temporarily unavailable (502 Bad Gateway)"
	case http.StatusServiceUnavailable:
		return code, "Service is temporarily unavailable (503)"
	case http.StatusGatewayTimeout:
		return code, "Service request timed out (504)"
	case http.StatusUnauthorized:
		return code, "Invalid API key"
	case http.StatusForbidden:
		return code, "Access forbidden"
	case http.StatusNotFound:
		return code, "Service endpoint not found"
	default:
		return code, fmt.Sprintf("Service returned error: %s (%d)", http.StatusText(code), code)
	}
}

// determineErrorResponse maps errors to appropriate HTTP status codes and user-friendly messages
func determineErrorResponse(err error) (int, string) {
	var maintErr *maintainerr.ErrMaintainerr
	if errors.As(err, &maintErr) {
		if maintErr.HttpCode > 0 {
			return handleHTTPStatusCode(maintErr.HttpCode)
		}

		// Handle specific error messages
		if maintErr.Op == "get_collections" && (maintErr.Error() == "maintainerr get_collections: URL is required" ||
			maintErr.Error() == "maintainerr get_collections: API key is required") {
			return http.StatusBadRequest, maintErr.Error()
		}

		switch {
		case strings.Contains(maintErr.Error(), "failed to connect"):
			return http.StatusServiceUnavailable, "Unable to connect to service"
		case strings.Contains(maintErr.Error(), "failed to read response"):
			return http.StatusBadGateway, "Invalid response from service"
		case strings.Contains(maintErr.Error(), "failed to parse response"):
			return http.StatusUnprocessableEntity, "Unable to process service response"
		}
	}

	if err == context.DeadlineExceeded || err == context.Canceled {
		return http.StatusGatewayTimeout, "Request timed out"
	}

	return http.StatusInternalServerError, "Internal server error"
}

func (h *MaintainerrHandler) GetMaintainerrCollections(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Maintainerr] No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	// Verify this is a Maintainerr instance
	if instanceId[:11] != "maintainerr" {
		log.Error().Str("instanceId", instanceId).Msg("[Maintainerr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Maintainerr instance ID"})
		return
	}

	cacheKey := maintainerrCachePrefix + instanceId
	ctx := context.Background()

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("collections:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
			return h.fetchCollections(ctx, instanceId)
		})
	})

	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			c.JSON(http.StatusOK, []maintainerr.Collection{})
			return
		}

		status, message := determineErrorResponse(err)
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Int("status", status).
			Str("message", message).
			Msg("[Maintainerr] Failed to fetch collections")

		c.JSON(status, gin.H{
			"error": message,
			"code":  status,
		})
		return
	}

	collections := result.([]maintainerr.Collection)

	// Add change detection logging
	h.compareAndLogCollectionChanges(instanceId, collections)

	// Broadcast collections update via SSE
	h.broadcastMaintainerrCollections(instanceId, collections)

	c.JSON(http.StatusOK, collections)
}

func (h *MaintainerrHandler) fetchCollections(ctx context.Context, instanceId string) ([]maintainerr.Collection, error) {
	// Create a child context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	maintainerrConfig, err := h.db.FindServiceBy(timeoutCtx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, fmt.Errorf("failed to get service config: %w", err)
	}

	if maintainerrConfig == nil || maintainerrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &maintainerr.MaintainerrService{}
	collections, err := service.GetCollections(timeoutCtx, maintainerrConfig.URL, maintainerrConfig.APIKey)
	if err != nil {
		return nil, err // Pass through the ErrMaintainerr
	}

	return collections, nil
}

// createCollectionsHash generates a unique hash representing the current Maintainerr collections
func createCollectionsHash(collections []maintainerr.Collection) string {
	if len(collections) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, collection := range collections {
		fmt.Fprintf(&sb, "%d:%s:%d,",
			collection.ID,
			collection.Title,
			len(collection.Media))
	}
	return sb.String()
}

// detectCollectionChanges determines the type of change in collections
func (h *MaintainerrHandler) detectCollectionChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_collections"
	}

	oldCollections := strings.Split(oldHash, ",")
	newCollections := strings.Split(newHash, ",")

	if len(oldCollections) < len(newCollections) {
		return "collection_added"
	} else if len(oldCollections) > len(newCollections) {
		return "collection_removed"
	}

	return "collection_updated"
}

// compareAndLogCollectionChanges tracks and logs changes in Maintainerr collections
func (h *MaintainerrHandler) compareAndLogCollectionChanges(instanceId string, collections []maintainerr.Collection) {
	h.lastCollectionsHashMu.Lock()
	defer h.lastCollectionsHashMu.Unlock()

	currentHash := createCollectionsHash(collections)
	lastHash := h.lastCollectionsHash[instanceId]

	if currentHash != lastHash {
		// Detect specific changes
		changes := h.detectCollectionChanges(lastHash, currentHash)

		log.Debug().
			Str("instanceId", instanceId).
			Int("count", len(collections)).
			Str("change", changes).
			Msg("[Maintainerr] Collections changed")

		h.lastCollectionsHash[instanceId] = currentHash
	}
}

// broadcastMaintainerrCollections broadcasts collections updates to all connected SSE clients
func (h *MaintainerrHandler) broadcastMaintainerrCollections(instanceId string, collections []maintainerr.Collection) {
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "maintainerr_collections",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"maintainerr": collections,
		},
		Details: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collectionCount": len(collections),
			},
		},
	})
}
