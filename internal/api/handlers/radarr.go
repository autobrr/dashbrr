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
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	radarrCacheDuration = 5 * time.Second
	radarrQueuePrefix   = "radarr:queue:"
)

type RadarrHandler struct {
	db    *database.DB
	cache cache.Store
}

func NewRadarrHandler(db *database.DB, cache cache.Store) *RadarrHandler {
	return &RadarrHandler{
		db:    db,
		cache: cache,
	}
}

// GrabQueueItem handles grabbing/retrying a queue item
// func (h *RadarrHandler) GrabQueueItem(c *gin.Context) {
// 	instanceId := c.Query("instanceId")
// 	if instanceId == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
// 		return
// 	}

// 	movieId := c.Query("movieId")
// 	if movieId == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "movieId is required"})
// 		return
// 	}

// 	path := c.Query("path")
// 	if path == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
// 		return
// 	}

// 	// Convert movieId to int
// 	movieIdInt, err := strconv.Atoi(movieId)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid movieId"})
// 		return
// 	}

// 	radarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
// 	if err != nil {
// 		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Radarr configuration")
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Radarr configuration"})
// 		return
// 	}

// 	if radarrConfig == nil {
// 		log.Error().Str("instanceId", instanceId).Msg("Radarr is not configured")
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Radarr is not configured"})
// 		return
// 	}

// 	// Create Radarr service instance
// 	service := &radarr.RadarrService{}

// 	// Call the service method to grab the queue item
// 	if err := service.GrabQueueItem(radarrConfig.URL, radarrConfig.APIKey, movieIdInt, path); err != nil {
// 		if radarrErr, ok := err.(*radarr.ErrRadarr); ok {
// 			log.Error().
// 				Err(radarrErr).
// 				Str("instanceId", instanceId).
// 				Int("movieId", movieIdInt).
// 				Str("path", path).
// 				Msg("Failed to grab queue item")

// 			if radarrErr.HttpCode > 0 {
// 				c.JSON(radarrErr.HttpCode, gin.H{"error": radarrErr.Error()})
// 				return
// 			}
// 		}
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to grab queue item: %v", err)})
// 		return
// 	}

// 	// Clear cache after successful grab
// 	cacheKey := radarrQueuePrefix + instanceId
// 	h.cache.Delete(context.Background(), cacheKey)

// 	c.JSON(http.StatusOK, gin.H{"message": "Queue item grabbed successfully"})
// }

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
	var queueResp types.RadarrQueueResponse
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

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Get queue records using the service
	records, err := service.GetQueue(radarrConfig.URL, radarrConfig.APIKey)
	if err != nil {
		if radarrErr, ok := err.(*radarr.ErrRadarr); ok {
			log.Error().
				Err(radarrErr).
				Str("instanceId", instanceId).
				Msg("Failed to fetch Radarr queue")

			if radarrErr.HttpCode > 0 {
				c.JSON(radarrErr.HttpCode, gin.H{"error": radarrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	// Create response
	queueResp = types.RadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
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

// DeleteQueueItem handles the deletion of a queue item with specified options
func (h *RadarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	// Get options from query parameters
	options := types.RadarrQueueDeleteOptions{
		RemoveFromClient: c.Query("removeFromClient") == "true",
		Blocklist:        c.Query("blocklist") == "true",
		SkipRedownload:   c.Query("skipRedownload") == "true",
		ChangeCategory:   c.Query("changeCategory") == "true",
	}

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

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Call the service method to delete the queue item
	if err := service.DeleteQueueItem(radarrConfig.URL, radarrConfig.APIKey, queueId, options); err != nil {
		if radarrErr, ok := err.(*radarr.ErrRadarr); ok {
			log.Error().
				Err(radarrErr).
				Str("instanceId", instanceId).
				Str("queueId", queueId).
				Msg("Failed to delete queue item")

			if radarrErr.HttpCode > 0 {
				c.JSON(radarrErr.HttpCode, gin.H{"error": radarrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
		return
	}

	// Clear cache after successful deletion
	cacheKey := radarrQueuePrefix + instanceId
	h.cache.Delete(context.Background(), cacheKey)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}
