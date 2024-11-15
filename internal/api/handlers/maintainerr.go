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

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	defaultTimeout = 10 * time.Second
	cacheDuration  = 30 * time.Second
	cachePrefix    = "maintainerr:collections:"
)

type MaintainerrHandler struct {
	db    *database.DB
	cache cache.Store
	sf    *singleflight.Group

	lastCollectionsHash   map[string]string
	lastCollectionsHashMu sync.Mutex
}

func NewMaintainerrHandler(db *database.DB, cache cache.Store) *MaintainerrHandler {
	return &MaintainerrHandler{
		db:                  db,
		cache:               cache,
		sf:                  &singleflight.Group{},
		lastCollectionsHash: make(map[string]string),
	}
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

		log.Info().
			Str("instanceId", instanceId).
			Int("count", len(collections)).
			Str("change", changes).
			Msg("Maintainerr collections changed")

		h.lastCollectionsHash[instanceId] = currentHash
	}
}

func (h *MaintainerrHandler) GetMaintainerrCollections(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	cacheKey := cachePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var collections []maintainerr.Collection
	err := h.cache.Get(ctx, cacheKey, &collections)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("count", len(collections)).
			Msg("Serving Maintainerr collections from cache")
		c.JSON(http.StatusOK, collections)

		// Refresh cache in background if needed
		go h.refreshCollectionsCache(instanceId, cacheKey)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("collections:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheCollections(ctx, instanceId, cacheKey)
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
			Msg("Failed to fetch Maintainerr collections")

		c.JSON(status, gin.H{
			"error": message,
			"code":  status,
		})
		return
	}

	collections = result.([]maintainerr.Collection)

	// Add change detection logging
	h.compareAndLogCollectionChanges(instanceId, collections)

	h.lastCollectionsHashMu.Lock()
	currentHash := createCollectionsHash(collections)
	lastHash := h.lastCollectionsHash[instanceId]

	// Only log when there are collections and the hash has changed
	if (lastHash == "" || currentHash != lastHash) && len(collections) > 0 {
		log.Info().
			Int("count", len(collections)).
			Str("instanceId", instanceId).
			Msg("[Maintainerr] Successfully retrieved and cached collections")
	}

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Maintainerr collections changed")
		h.lastCollectionsHash[instanceId] = currentHash
	}
	h.lastCollectionsHashMu.Unlock()

	c.JSON(http.StatusOK, collections)
}

func (h *MaintainerrHandler) fetchAndCacheCollections(ctx context.Context, instanceId, cacheKey string) ([]maintainerr.Collection, error) {
	// Create a child context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
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

	// Only cache successful responses
	if err := h.cache.Set(timeoutCtx, cacheKey, collections, cacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Maintainerr collections")
	}

	return collections, nil
}

func (h *MaintainerrHandler) refreshCollectionsCache(instanceId, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	// Use singleflight for refresh operations as well
	sfKey := fmt.Sprintf("collections_refresh:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheCollections(context.Background(), instanceId, cacheKey)
	})

	if err != nil {
		if err.Error() != "service not configured" {
			status, message := determineErrorResponse(err)
			log.Error().
				Err(err).
				Str("instanceId", instanceId).
				Int("status", status).
				Str("message", message).
				Msg("Failed to refresh Maintainerr collections cache")
		}
		return
	}

	collections := result.([]maintainerr.Collection)

	// Add change detection logging
	h.compareAndLogCollectionChanges(instanceId, collections)

	h.lastCollectionsHashMu.Lock()
	currentHash := createCollectionsHash(collections)
	lastHash := h.lastCollectionsHash[instanceId]

	// Only log when there are collections and the hash has changed
	if (lastHash == "" || currentHash != lastHash) && len(collections) > 0 {
		log.Info().
			Str("instanceId", instanceId).
			Int("count", len(collections)).
			Msg("Successfully refreshed Maintainerr collections cache")
	}

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Maintainerr collections changed")
		h.lastCollectionsHash[instanceId] = currentHash
	}
	h.lastCollectionsHashMu.Unlock()
}
