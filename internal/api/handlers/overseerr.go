// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
)

const (
	overseerrCacheDuration = 5 * time.Minute
	overseerrCachePrefix   = "overseerr:requests:"
)

type OverseerrHandler struct {
	db    *database.DB
	cache cache.Store
}

func NewOverseerrHandler(db *database.DB, cache cache.Store) *OverseerrHandler {
	return &OverseerrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *OverseerrHandler) GetRequests(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	cacheKey := overseerrCachePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var response *overseerr.RequestsStats
	err := h.cache.Get(ctx, cacheKey, &response)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("pendingCount", response.PendingCount).
			Int("totalRequests", len(response.Requests)).
			Msg("Serving Overseerr requests from cache")
		c.JSON(http.StatusOK, response)

		// Refresh cache in background if needed
		go h.refreshRequestsCache(instanceId, cacheKey)
		return
	}

	// If not in cache, fetch from service
	stats, err := h.fetchAndCacheRequests(instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			c.JSON(http.StatusOK, &overseerr.RequestsStats{
				PendingCount: 0,
				Requests:     []overseerr.MediaRequest{},
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

	log.Debug().
		Str("instanceId", instanceId).
		Int("pendingCount", stats.PendingCount).
		Int("totalRequests", len(stats.Requests)).
		Msg("Successfully retrieved and cached Overseerr requests")

	c.JSON(http.StatusOK, stats)
}

func (h *OverseerrHandler) fetchAndCacheRequests(instanceId, cacheKey string) (*overseerr.RequestsStats, error) {
	overseerrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		return nil, err
	}

	if overseerrConfig == nil || overseerrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &overseerr.OverseerrService{}
	service.SetDB(h.db) // Set the database instance for fetching Radarr/Sonarr configs

	stats, err := service.GetRequests(overseerrConfig.URL, overseerrConfig.APIKey)
	if err != nil {
		return nil, err
	}

	// Cache the results
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, stats, overseerrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Overseerr requests")
	}

	return stats, nil
}

func (h *OverseerrHandler) refreshRequestsCache(instanceId, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	stats, err := h.fetchAndCacheRequests(instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Overseerr requests cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("pendingCount", stats.PendingCount).
		Int("totalRequests", len(stats.Requests)).
		Msg("Successfully refreshed Overseerr requests cache")
}
