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

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	sonarrCacheDuration = 5 * time.Second
	sonarrQueuePrefix   = "sonarr:queue:"
	sonarrStatsPrefix   = "sonarr:stats:"
)

type SonarrHandler struct {
	db    *database.DB
	cache cache.Store
}

func NewSonarrHandler(db *database.DB, cache cache.Store) *SonarrHandler {
	return &SonarrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *SonarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Sonarr instance
	if instanceId[:6] != "sonarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Sonarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		log.Error().Msg("No queue ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue ID is required"})
		return
	}

	// Get delete options from query parameters
	removeFromClient := c.Query("removeFromClient") == "true"
	blocklist := c.Query("blocklist") == "true"
	skipRedownload := c.Query("skipRedownload") == "true"
	changeCategory := c.Query("changeCategory") == "true"

	// Get Sonarr configuration
	sonarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Sonarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Sonarr configuration"})
		return
	}

	if sonarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Sonarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Sonarr is not configured"})
		return
	}

	// Build delete URL with query parameters
	deleteURL := fmt.Sprintf("%s/api/v3/queue/%s?removeFromClient=%t&blocklist=%t&skipRedownload=%t",
		sonarrConfig.URL,
		queueId,
		removeFromClient,
		blocklist,
		skipRedownload)

	if changeCategory {
		deleteURL += "&changeCategory=true"
	}

	// Create DELETE request
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create delete request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create delete request"})
		return
	}

	// Add API key header
	req.Header.Add("X-Api-Key", sonarrConfig.APIKey)

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute delete request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute delete request"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err == nil && errorResponse.Message != "" {
			log.Error().
				Str("message", errorResponse.Message).
				Int("statusCode", resp.StatusCode).
				Msg("Sonarr API returned error")
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse.Message})
			return
		}
		log.Error().
			Int("statusCode", resp.StatusCode).
			Msg("Sonarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Sonarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Clear queue cache for this instance
	cacheKey := sonarrQueuePrefix + instanceId
	if err := h.cache.Delete(context.Background(), cacheKey); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to clear Sonarr queue cache")
	}

	log.Info().
		Str("instanceId", instanceId).
		Str("queueId", queueId).
		Bool("removeFromClient", removeFromClient).
		Bool("blocklist", blocklist).
		Bool("skipRedownload", skipRedownload).
		Bool("changeCategory", changeCategory).
		Msg("Successfully deleted queue item")

	c.Status(http.StatusOK)
}

func (h *SonarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Sonarr instance
	if instanceId[:6] != "sonarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Sonarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrQueuePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var queueResp types.SonarrQueueResponse
	err := h.cache.Get(ctx, cacheKey, &queueResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Serving Sonarr queue from cache")
		c.JSON(http.StatusOK, queueResp)
		return
	}

	// If not in cache, fetch from service
	sonarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Sonarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Sonarr configuration"})
		return
	}

	if sonarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Sonarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Sonarr is not configured"})
		return
	}

	// Build Sonarr API URL
	apiURL := fmt.Sprintf("%s/api/v3/queue?apikey=%s", sonarrConfig.URL, sonarrConfig.APIKey)

	// Make request to Sonarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Sonarr queue")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Sonarr queue"})
		return
	}

	if resp == nil {
		log.Error().Str("instanceId", instanceId).Msg("Received nil response from Sonarr")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received nil response from Sonarr"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Sonarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Sonarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Sonarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Sonarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, queueResp, sonarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr queue")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("totalRecords", queueResp.TotalRecords).
		Msg("Successfully retrieved and cached Sonarr queue")

	c.JSON(http.StatusOK, queueResp)
}

func (h *SonarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Sonarr instance
	if instanceId[:6] != "sonarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Sonarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrStatsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var statsResp types.SonarrStatsResponse
	err := h.cache.Get(ctx, cacheKey, &statsResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("monitored", statsResp.Monitored).
			Msg("Serving Sonarr stats from cache")
		c.JSON(http.StatusOK, statsResp)
		return
	}

	// If not in cache, fetch from service
	sonarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Sonarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Sonarr configuration"})
		return
	}

	if sonarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Sonarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Sonarr is not configured"})
		return
	}

	// Build Sonarr API URL
	apiURL := fmt.Sprintf("%s/api/v3/system/status?apikey=%s", sonarrConfig.URL, sonarrConfig.APIKey)

	// Make request to Sonarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Sonarr stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Sonarr stats"})
		return
	}

	if resp == nil {
		log.Error().Str("instanceId", instanceId).Msg("Received nil response from Sonarr")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received nil response from Sonarr"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Sonarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Sonarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Sonarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Sonarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, statsResp, sonarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr stats")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("monitored", statsResp.Monitored).
		Msg("Successfully retrieved and cached Sonarr stats")

	c.JSON(http.StatusOK, statsResp)
}
