// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/database"
	"github.com/autobrr/dashbrr/backend/services/cache"
	"github.com/autobrr/dashbrr/backend/types"
)

const (
	prowlarrCacheDuration = 60 * time.Second
	prowlarrStatsPrefix   = "prowlarr:stats:"
	prowlarrIndexerPrefix = "prowlarr:indexers:"
)

type ProwlarrHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewProwlarrHandler(db *database.DB, cache *cache.Cache) *ProwlarrHandler {
	return &ProwlarrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *ProwlarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Prowlarr instance
	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Prowlarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrStatsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var statsResp types.ProwlarrStatsResponse
	err := h.cache.Get(ctx, cacheKey, &statsResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("grabCount", statsResp.GrabCount).
			Msg("Serving Prowlarr stats from cache")
		c.JSON(http.StatusOK, statsResp)
		return
	}

	// If not in cache, fetch from service
	prowlarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Prowlarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Prowlarr configuration"})
		return
	}

	if prowlarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Prowlarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Prowlarr is not configured"})
		return
	}

	// Build Prowlarr API URL
	apiURL := fmt.Sprintf("%s/api/v1/system/status?apikey=%s", prowlarrConfig.URL, prowlarrConfig.APIKey)

	// Make request to Prowlarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Prowlarr stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Prowlarr stats"})
		return
	}

	if resp == nil {
		log.Error().Str("instanceId", instanceId).Msg("Received nil response from Prowlarr")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received nil response from Prowlarr"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Prowlarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Prowlarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Prowlarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Prowlarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, statsResp, prowlarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Prowlarr stats")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("grabCount", statsResp.GrabCount).
		Msg("Successfully retrieved and cached Prowlarr stats")

	c.JSON(http.StatusOK, statsResp)
}

func (h *ProwlarrHandler) GetIndexers(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Prowlarr instance
	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Prowlarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrIndexerPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var indexers []types.ProwlarrIndexer
	err := h.cache.Get(ctx, cacheKey, &indexers)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(indexers)).
			Msg("Serving Prowlarr indexers from cache")
		c.JSON(http.StatusOK, indexers)
		return
	}

	// If not in cache, fetch from service
	prowlarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Prowlarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Prowlarr configuration"})
		return
	}

	if prowlarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Prowlarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Prowlarr is not configured"})
		return
	}

	// Build Prowlarr API URL
	apiURL := fmt.Sprintf("%s/api/v1/indexer?apikey=%s", prowlarrConfig.URL, prowlarrConfig.APIKey)

	// Make request to Prowlarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Prowlarr indexers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Prowlarr indexers"})
		return
	}

	if resp == nil {
		log.Error().Str("instanceId", instanceId).Msg("Received nil response from Prowlarr")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received nil response from Prowlarr"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Prowlarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Prowlarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&indexers); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Prowlarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Prowlarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, indexers, prowlarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Prowlarr indexers")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("indexerCount", len(indexers)).
		Msg("Successfully retrieved and cached Prowlarr indexers")

	c.JSON(http.StatusOK, indexers)
}
