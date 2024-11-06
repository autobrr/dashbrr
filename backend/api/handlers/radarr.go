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
)

const (
	radarrCacheDuration = 5 * time.Second
	radarrQueuePrefix   = "radarr:queue:"
)

// RadarrQueueResponse represents the queue response from Radarr API
type RadarrQueueResponse struct {
	TotalRecords int            `json:"totalRecords"`
	Records      []RadarrRecord `json:"records"`
}

type RadarrRecord struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Status            string `json:"status"`
	TimeLeft          string `json:"timeleft,omitempty"`
	Indexer           string `json:"indexer"`
	DownloadClient    string `json:"downloadClient"`
	CustomFormatScore int    `json:"customFormatScore"`
}

type RadarrHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewRadarrHandler(db *database.DB, cache *cache.Cache) *RadarrHandler {
	return &RadarrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *RadarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Radarr instance
	if instanceId[:6] != "radarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Radarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Radarr instance ID"})
		return
	}

	cacheKey := radarrQueuePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var queueResp RadarrQueueResponse
	err := h.cache.Get(ctx, cacheKey, &queueResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Serving Radarr queue from cache")
		c.JSON(http.StatusOK, queueResp)
		return
	}

	// If not in cache, fetch from service
	radarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Radarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Radarr configuration"})
		return
	}

	if radarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Radarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Radarr is not configured"})
		return
	}

	// Build Radarr API URL
	apiURL := fmt.Sprintf("%s/api/v3/queue?apikey=%s", radarrConfig.URL, radarrConfig.APIKey)

	// Make request to Radarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Radarr queue")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Radarr queue"})
		return
	}

	if resp == nil {
		log.Error().Str("instanceId", instanceId).Msg("Received nil response from Radarr")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received nil response from Radarr"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Radarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Radarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Radarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Radarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, queueResp, radarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Radarr queue")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("totalRecords", queueResp.TotalRecords).
		Msg("Successfully retrieved and cached Radarr queue")

	c.JSON(http.StatusOK, queueResp)
}
