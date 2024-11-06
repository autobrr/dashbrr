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
	"github.com/autobrr/dashbrr/backend/services/plex"
)

const (
	plexCacheDuration = 5 * time.Second // Reduced from 30s to 5s for more frequent updates
	plexCachePrefix   = "plex:sessions:"
)

type PlexHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewPlexHandler(db *database.DB, cache *cache.Cache) *PlexHandler {
	return &PlexHandler{
		db:    db,
		cache: cache,
	}
}

func (h *PlexHandler) GetPlexSessions(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Plex instance
	if instanceId[:4] != "plex" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Plex instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Plex instance ID"})
		return
	}

	cacheKey := plexCachePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var sessions *plex.PlexSessionsResponse
	err := h.cache.Get(ctx, cacheKey, &sessions)
	if err == nil && sessions != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Msg("Serving Plex sessions from cache")
		c.JSON(http.StatusOK, sessions)

		// Refresh cache in background without delay
		go h.refreshSessionsCache(instanceId, cacheKey)
		return
	}

	// If not in cache or invalid cache data, fetch from service
	sessions, err = h.fetchAndCacheSessions(instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			emptyResponse := &plex.PlexSessionsResponse{}
			emptyResponse.MediaContainer.Size = 0
			emptyResponse.MediaContainer.Metadata = []plex.PlexSession{}
			c.JSON(http.StatusOK, emptyResponse)
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Plex sessions")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Plex sessions")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if sessions != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Msg("Successfully retrieved and cached Plex sessions")
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Retrieved empty Plex sessions")
	}

	c.JSON(http.StatusOK, sessions)
}

func (h *PlexHandler) fetchAndCacheSessions(instanceId, cacheKey string) (*plex.PlexSessionsResponse, error) {
	plexConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		return nil, err
	}

	if plexConfig == nil || plexConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &plex.PlexService{}
	sessions, err := service.GetSessions(plexConfig.URL, plexConfig.APIKey)
	if err != nil {
		return nil, err
	}

	if sessions == nil {
		return nil, nil
	}

	// Initialize empty metadata if nil
	if sessions.MediaContainer.Metadata == nil {
		sessions.MediaContainer.Metadata = []plex.PlexSession{}
	}

	// Cache the results
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, sessions, plexCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Plex sessions")
	}

	return sessions, nil
}

func (h *PlexHandler) refreshSessionsCache(instanceId, cacheKey string) {
	sessions, err := h.fetchAndCacheSessions(instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Plex sessions cache")
		return
	}

	if sessions != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Msg("Successfully refreshed Plex sessions cache")
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Refreshed cache with empty Plex sessions")
	}
}
