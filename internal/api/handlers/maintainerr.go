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
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
)

const (
	defaultTimeout = 10 * time.Second
	cacheDuration  = 30 * time.Second
	cachePrefix    = "maintainerr:collections:"
)

type MaintainerrHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewMaintainerrHandler(db *database.DB, cache *cache.Cache) *MaintainerrHandler {
	return &MaintainerrHandler{
		db:    db,
		cache: cache,
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

	// If not in cache, fetch from service
	collections, err = h.fetchAndCacheCollections(instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			c.JSON(http.StatusOK, []maintainerr.Collection{})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Maintainerr collections")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Maintainerr collections")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	log.Debug().
		Int("count", len(collections)).
		Str("instanceId", instanceId).
		Msg("Successfully retrieved and cached Maintainerr collections")

	c.JSON(http.StatusOK, collections)
}

func (h *MaintainerrHandler) fetchAndCacheCollections(instanceId, cacheKey string) ([]maintainerr.Collection, error) {
	maintainerrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		return nil, err
	}

	if maintainerrConfig == nil || maintainerrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &maintainerr.MaintainerrService{}
	collections, err := service.GetCollections(maintainerrConfig.URL, maintainerrConfig.APIKey)
	if err != nil {
		return nil, err
	}

	// Cache the results
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, collections, cacheDuration); err != nil {
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

	collections, err := h.fetchAndCacheCollections(instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Maintainerr collections cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("count", len(collections)).
		Msg("Successfully refreshed Maintainerr collections cache")
}
