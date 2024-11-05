package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/database"
	"github.com/autobrr/dashbrr/backend/services/cache"
	"github.com/autobrr/dashbrr/backend/services/overseerr"
)

const (
	overseerrCacheDuration = 30 * time.Second
	overseerrCachePrefix   = "overseerr:requests:"
)

type OverseerrHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewOverseerrHandler(db *database.DB, cache *cache.Cache) *OverseerrHandler {
	return &OverseerrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *OverseerrHandler) GetPendingRequests(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	cacheKey := overseerrCachePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var response struct {
		PendingRequests int `json:"pendingRequests"`
	}
	err := h.cache.Get(ctx, cacheKey, &response)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("count", response.PendingRequests).
			Msg("Serving Overseerr pending requests from cache")
		c.JSON(http.StatusOK, response)

		// Refresh cache in background if needed
		go h.refreshRequestsCache(instanceId, cacheKey)
		return
	}

	// If not in cache, fetch from service
	count, err := h.fetchAndCacheRequests(instanceId, cacheKey)
	if err != nil {
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

	log.Debug().
		Str("instanceId", instanceId).
		Int("count", count).
		Msg("Successfully retrieved and cached Overseerr pending requests")

	c.JSON(http.StatusOK, gin.H{
		"pendingRequests": count,
	})
}

func (h *OverseerrHandler) fetchAndCacheRequests(instanceId, cacheKey string) (int, error) {
	overseerrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		return 0, err
	}

	if overseerrConfig == nil {
		return 0, fmt.Errorf("overseerr is not configured")
	}

	service := &overseerr.OverseerrService{}
	count, err := service.GetPendingRequests(overseerrConfig.URL, overseerrConfig.APIKey)
	if err != nil {
		return 0, err
	}

	// Cache the results
	ctx := context.Background()
	response := struct {
		PendingRequests int `json:"pendingRequests"`
	}{
		PendingRequests: count,
	}
	if err := h.cache.Set(ctx, cacheKey, response, overseerrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Overseerr pending requests")
	}

	return count, nil
}

func (h *OverseerrHandler) refreshRequestsCache(instanceId, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	count, err := h.fetchAndCacheRequests(instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Overseerr requests cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("count", count).
		Msg("Successfully refreshed Overseerr requests cache")
}
