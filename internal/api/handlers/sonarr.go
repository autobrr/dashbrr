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

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	sonarrQueuePrefix = "sonarr:queue:"
	sonarrStatsPrefix = "sonarr:stats:"
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
	options := types.SonarrQueueDeleteOptions{
		RemoveFromClient: c.Query("removeFromClient") == "true",
		Blocklist:        c.Query("blocklist") == "true",
		SkipRedownload:   c.Query("skipRedownload") == "true",
		ChangeCategory:   c.Query("changeCategory") == "true",
	}

	// Get Sonarr configuration
	sonarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
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

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Call the service method to delete the queue item
	if err := service.DeleteQueueItem(context.Background(), sonarrConfig.URL, sonarrConfig.APIKey, queueId, options); err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Str("queueId", queueId).
				Msg("Failed to delete queue item")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
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

	// Fetch fresh data and broadcast update
	queueResp, err := h.fetchAndCacheQueue(instanceId, cacheKey)
	if err == nil {
		h.broadcastSonarrQueue(instanceId, &queueResp)
	}

	log.Info().
		Str("instanceId", instanceId).
		Str("queueId", queueId).
		Bool("removeFromClient", options.RemoveFromClient).
		Bool("blocklist", options.Blocklist).
		Bool("skipRedownload", options.SkipRedownload).
		Bool("changeCategory", options.ChangeCategory).
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

		// Refresh cache in background without delay
		go h.refreshQueueCache(instanceId, cacheKey)
		return
	}

	// If not in cache, fetch from service
	queueResp, err = h.fetchAndCacheQueue(instanceId, cacheKey)
	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("Failed to fetch Sonarr queue")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	if queueResp.Records != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Successfully retrieved and cached Sonarr queue")

		// Broadcast queue update via SSE
		h.broadcastSonarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Retrieved empty Sonarr queue")
	}

	c.JSON(http.StatusOK, queueResp)
}

func (h *SonarrHandler) fetchAndCacheQueue(instanceId, cacheKey string) (types.SonarrQueueResponse, error) {
	sonarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.SonarrQueueResponse{}, err
	}

	if sonarrConfig == nil {
		return types.SonarrQueueResponse{}, fmt.Errorf("sonarr is not configured")
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Get queue records using the service
	records, err := service.GetQueueForHealth(context.Background(), sonarrConfig.URL, sonarrConfig.APIKey)
	if err != nil {
		return types.SonarrQueueResponse{}, err
	}

	// Ensure Episodes array is populated for each record
	for i := range records {
		if records[i].Episode != (types.Episode{}) {
			records[i].Episodes = []types.EpisodeBasic{{
				ID:            records[i].Episode.ID,
				EpisodeNumber: records[i].Episode.EpisodeNumber,
				SeasonNumber:  records[i].Episode.SeasonNumber,
			}}
		}
	}

	// Create response
	queueResp := types.SonarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	// Cache the results using the centralized cache duration
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, queueResp, middleware.CacheDurations.SonarrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr queue")
	}

	return queueResp, nil
}

func (h *SonarrHandler) refreshQueueCache(instanceId, cacheKey string) {
	queueResp, err := h.fetchAndCacheQueue(instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Sonarr queue cache")
		return
	}

	if queueResp.Records != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Successfully refreshed Sonarr queue cache")

		// Broadcast queue update via SSE
		h.broadcastSonarrQueue(instanceId, &queueResp)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Refreshed cache with empty Sonarr queue")
	}
}

// broadcastSonarrQueue broadcasts Sonarr queue updates to all connected SSE clients
func (h *SonarrHandler) broadcastSonarrQueue(instanceId string, queueResp *types.SonarrQueueResponse) {
	// Calculate additional statistics
	var totalSize int64
	var downloading int
	var episodeCount int
	for _, record := range queueResp.Records {
		totalSize += record.Size
		if record.Status == "downloading" {
			downloading++
		}
		episodeCount += len(record.Episodes)
	}

	// Use the existing BroadcastHealth function with a special message type
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "ok",
		Message:     "sonarr_queue",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"sonarr": queueResp,
		},
		Details: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queueCount":       queueResp.TotalRecords,
				"totalRecords":     queueResp.TotalRecords,
				"downloadingCount": downloading,
				"episodeCount":     episodeCount,
				"totalSize":        totalSize,
			},
		},
	})
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
		c.JSON(http.StatusOK, gin.H{
			"stats":   statsResp,
			"version": "", // Version will be added by the frontend if needed
		})

		// Refresh cache in background without delay
		go h.refreshStatsCache(instanceId, cacheKey)
		return
	}

	// If not in cache, fetch from service
	statsResp, version, err := h.fetchAndCacheStats(instanceId, cacheKey)
	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("Failed to fetch Sonarr stats")

			if arrErr.HttpCode > 0 {
				c.JSON(arrErr.HttpCode, gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch stats: %v", err)})
		return
	}

	// Broadcast stats update via SSE
	h.broadcastSonarrStats(instanceId, &statsResp, version)

	// Create response with stats and version
	c.JSON(http.StatusOK, gin.H{
		"stats":   statsResp,
		"version": version,
	})
}

func (h *SonarrHandler) fetchAndCacheStats(instanceId, cacheKey string) (types.SonarrStatsResponse, string, error) {
	sonarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.SonarrStatsResponse{}, "", err
	}

	if sonarrConfig == nil {
		return types.SonarrStatsResponse{}, "", fmt.Errorf("sonarr is not configured")
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Get system status using the service
	version, err := service.GetSystemStatus(sonarrConfig.URL, sonarrConfig.APIKey)
	if err != nil {
		return types.SonarrStatsResponse{}, "", err
	}

	// Cache the results using the centralized cache duration
	ctx := context.Background()
	if err := h.cache.Set(ctx, cacheKey, version, middleware.CacheDurations.SonarrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr stats")
	}

	return types.SonarrStatsResponse{}, version, nil
}

func (h *SonarrHandler) refreshStatsCache(instanceId, cacheKey string) {
	statsResp, version, err := h.fetchAndCacheStats(instanceId, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Sonarr stats cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("monitored", statsResp.Monitored).
		Msg("Successfully refreshed Sonarr stats cache")

	// Broadcast stats update via SSE
	h.broadcastSonarrStats(instanceId, &statsResp, version)
}

// broadcastSonarrStats broadcasts Sonarr stats updates to all connected SSE clients
func (h *SonarrHandler) broadcastSonarrStats(instanceId string, statsResp *types.SonarrStatsResponse, version string) {
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "ok",
		Message:     "sonarr_stats",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"stats":   statsResp,
				"version": version,
			},
		},
		Details: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"monitored":  statsResp.Monitored,
				"version":    version,
				"queueCount": statsResp.QueuedCount,
			},
		},
	})
}
