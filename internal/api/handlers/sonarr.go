// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
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
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	sonarrQueuePrefix       = "sonarr:queue:"
	sonarrStatsPrefix       = "sonarr:stats:"
	sonarrStaleDataDuration = 5 * time.Minute
)

type sonarrStatsResult struct {
	Stats   types.SonarrStatsResponse
	Version string
}

type SonarrHandler struct {
	db              *database.DB
	cache           cache.Store
	bc              *Broadcaster
	sf              *singleflight.Group
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastStatsHash   map[string]string
	lastQueueHashMu sync.Mutex
	lastStatsHashMu sync.Mutex
}

func NewSonarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *SonarrHandler {
	return &SonarrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		sf:             &singleflight.Group{},
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastQueueHash:  make(map[string]string),
		lastStatsHash:  make(map[string]string),
	}
}

func (h *SonarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Sonarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if !strings.HasPrefix(instanceId, "sonarr") {
		log.Error().Str("instanceId", instanceId).Msg("[Sonarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrQueuePrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.SonarrQueueResponse]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.SonarrStatus,
		StaleTTL:       sonarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() (types.SonarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceId)
		},
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("[Sonarr] Failed to fetch queue")

			if arrErr.HttpCode > 0 {
				c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch queue: %v", err)})
		return
	}

	if len(result.Records) > 0 {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", result.TotalRecords).
			Msg("[Sonarr] Queue retrieved with records")

		// Add hash-based change detection
		h.compareAndLogQueueChanges(instanceId, &result)

		// Broadcast queue update via SSE
		h.broadcastSonarrQueue(instanceId, &result)
	}

	c.JSON(http.StatusOK, result)
}

func (h *SonarrHandler) fetchQueue(ctx context.Context, instanceId string) (types.SonarrQueueResponse, error) {
	sonarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.SonarrQueueResponse{}, err
	}

	if sonarrConfig == nil {
		return types.SonarrQueueResponse{}, NewServiceNotConfigured("sonarr")
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Get queue records using the service
	records, err := service.GetQueueForHealth(ctx, sonarrConfig.URL, sonarrConfig.APIKey)
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
	return types.SonarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}, nil
}

func (h *SonarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Sonarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if !strings.HasPrefix(instanceId, "sonarr") {
		log.Error().Str("instanceId", instanceId).Msg("[Sonarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrStatsPrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[sonarrStatsResult]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.SonarrStatus,
		StaleTTL:       sonarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() (sonarrStatsResult, error) {
			return h.fetchStats(ctx, instanceId)
		},
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Msg("[Sonarr] Failed to fetch stats")

			if arrErr.HttpCode > 0 {
				c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch stats: %v", err)})
		return
	}

	// Add hash-based change detection
	h.compareAndLogStatsChanges(instanceId, &result.Stats)

	// Broadcast stats update via SSE
	h.broadcastSonarrStats(instanceId, &result.Stats, result.Version)

	c.JSON(http.StatusOK, gin.H{
		"stats":   result.Stats,
		"version": result.Version,
	})
}

func (h *SonarrHandler) fetchStats(ctx context.Context, instanceId string) (sonarrStatsResult, error) {
	sonarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return sonarrStatsResult{}, err
	}

	if sonarrConfig == nil {
		return sonarrStatsResult{}, NewServiceNotConfigured("sonarr")
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Get system status using the service
	version, err := service.GetSystemStatus(ctx, sonarrConfig.URL, sonarrConfig.APIKey)
	if err != nil {
		return sonarrStatsResult{}, err
	}

	// Minimal stats: derive queue counts so the endpoint isn't a no-op.
	records, err := service.GetQueueForHealth(ctx, sonarrConfig.URL, sonarrConfig.APIKey)
	if err != nil {
		return sonarrStatsResult{}, err
	}

	episodeCount := 0
	for _, record := range records {
		episodeCount += len(record.Episodes)
	}

	return sonarrStatsResult{
		Stats: types.SonarrStatsResponse{
			QueuedCount:  len(records),
			EpisodeCount: episodeCount,
		},
		Version: version,
	}, nil
}

func (h *SonarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if !strings.HasPrefix(instanceId, "sonarr") {
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

	options := types.SonarrQueueDeleteOptions{
		RemoveFromClient: c.Query("removeFromClient") == "true",
		Blocklist:        c.Query("blocklist") == "true",
		SkipRedownload:   c.Query("skipRedownload") == "true",
		ChangeCategory:   c.Query("changeCategory") == "true",
	}

	ctx := c.Request.Context()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceId, queueId, options)
	})

	if err != nil {
		if arrErr, ok := err.(*arr.ErrArr); ok {
			log.Error().
				Err(arrErr).
				Str("instanceId", instanceId).
				Str("queueId", queueId).
				Msg("[Sonarr] Failed to delete queue item")

			if arrErr.HttpCode > 0 {
				c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
		return
	}

	// Clear cache after successful deletion
	cacheKey := sonarrQueuePrefix + instanceId
	if err := h.cache.Delete(ctx, cacheKey); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Sonarr] Failed to clear queue cache")
	}
	_ = h.cache.Delete(ctx, cacheKey+":stale")

	// Fetch fresh queue data
	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.SonarrQueueResponse]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.SonarrStatus,
		StaleTTL:       sonarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() (types.SonarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceId)
		},
	})

	if err == nil {
		h.broadcastSonarrQueue(instanceId, &result)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *SonarrHandler) deleteQueueItem(ctx context.Context, instanceId, queueId string, options types.SonarrQueueDeleteOptions) error {
	sonarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return err
	}

	if sonarrConfig == nil {
		return NewServiceNotConfigured("sonarr")
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(ctx, sonarrConfig.URL, sonarrConfig.APIKey, queueId, options)
}

// Helper methods for change detection
func (h *SonarrHandler) compareAndLogQueueChanges(instanceId string, queueResp *types.SonarrQueueResponse) {
	h.lastQueueHashMu.Lock()
	defer h.lastQueueHashMu.Unlock()

	wrapped := wrapSonarrQueue(queueResp)
	currentHash := generateQueueHash(wrapped)
	lastHash := h.lastQueueHash[instanceId]

	if currentHash != lastHash {
		changes := detectQueueChanges(lastHash, currentHash)
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Str("change", changes).
			Msg("[Sonarr] Queue changed")

		h.lastQueueHash[instanceId] = currentHash
	}
}

func (h *SonarrHandler) compareAndLogStatsChanges(instanceId string, stats *types.SonarrStatsResponse) {
	h.lastStatsHashMu.Lock()
	defer h.lastStatsHashMu.Unlock()

	currentHash := fmt.Sprintf("%d:%d:%d:%d",
		stats.EpisodeCount,
		stats.EpisodeFileCount,
		stats.QueuedCount,
		stats.MissingCount)
	lastHash := h.lastStatsHash[instanceId]

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Int("episodeCount", stats.EpisodeCount).
			Int("queuedCount", stats.QueuedCount).
			Msg("[Sonarr] Stats changed")

		h.lastStatsHash[instanceId] = currentHash
	}
}

func (h *SonarrHandler) broadcastSonarrQueue(instanceId string, queueResp *types.SonarrQueueResponse) {
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

	h.bc.Publish(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "sonarr_queue",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queue": queueResp,
			},
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

func (h *SonarrHandler) broadcastSonarrStats(instanceId string, statsResp *types.SonarrStatsResponse, version string) {
	h.bc.Publish(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
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
