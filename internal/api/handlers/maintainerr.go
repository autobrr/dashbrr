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
	bc             *Broadcaster
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastCollectionsHash   map[string]string
	lastCollectionsHashMu sync.Mutex
}

func NewMaintainerrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *MaintainerrHandler {
	return &MaintainerrHandler{
		db:                  db,
		cache:               cache,
		bc:                  bc,
		sf:                  &singleflight.Group{},
		circuitBreaker:      resilience.NewCircuitBreaker(5, 1*time.Minute),
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
		return http.StatusBadGateway, "Invalid API key"
	case http.StatusForbidden:
		return http.StatusBadGateway, "Access forbidden"
	case http.StatusNotFound:
		return http.StatusBadGateway, "Service endpoint not found"
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

		if maintErr.Op == "get_collections" && (errors.Is(maintErr, maintainerr.ErrURLRequired) ||
			errors.Is(maintErr, maintainerr.ErrAPIKeyRequired)) {
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
	if !strings.HasPrefix(instanceId, "maintainerr") {
		log.Error().Str("instanceId", instanceId).Msg("[Maintainerr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Maintainerr instance ID"})
		return
	}

	cacheKey := maintainerrCachePrefix + instanceId
	ctx := c.Request.Context()

	sfKey := fmt.Sprintf("collections:%s", instanceId)
	collections, err := FetchWithSWRCache(ctx, SWRCacheOptions[[]maintainerr.Collection]{
		Store:           h.cache,
		Key:             cacheKey,
		FreshTTL:        middleware.CacheDurations.MaintainerrStatus,
		StaleTTL:        maintainerrStaleDataDuration,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    h.sf,
		SingleflightKey: sfKey,
		Fetch: func() ([]maintainerr.Collection, error) {
			c, err := h.fetchCollections(ctx, instanceId)
			if err != nil {
				return nil, err
			}
			if c == nil {
				c = make([]maintainerr.Collection, 0)
			}
			return c, nil
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
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
		return nil, ErrServiceNotConfigured
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
	h.bc.Publish(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "maintainerr_collections",
		EventType:   models.ServiceEventInternal,
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collections": collections,
			},
		},
		Details: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collectionCount": len(collections),
			},
		},
	})
}
