// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
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
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastQueueHash:  make(map[string]string),
		lastStatsHash:  make(map[string]string),
	}
}

func (h *SonarrHandler) GetQueue(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "sonarr", "Sonarr")
	if !ok {
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
		if handleArrFetchError(c, err, "Sonarr", instanceId, "queue") {
			return
		}
		return
	}

	if len(result.Records) > 0 {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", result.TotalRecords).
			Msg("[Sonarr] Queue retrieved with records")
		compareAndLogArrQueueChanges(
			h.lastQueueHash,
			&h.lastQueueHashMu,
			"Sonarr",
			instanceId,
			result.TotalRecords,
			wrapSonarrQueue(&result),
		)
		publishInternalServiceUpdate(h.bc, buildSonarrQueueServiceUpdate(instanceId, &result))
	}

	c.JSON(http.StatusOK, result)
}

func (h *SonarrHandler) fetchQueue(ctx context.Context, instanceId string) (types.SonarrQueueResponse, error) {
	sonarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "sonarr")
	if err != nil {
		return types.SonarrQueueResponse{}, err
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
	instanceId, ok := requireInstanceID(c, "sonarr", "Sonarr")
	if !ok {
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
		if handleArrFetchError(c, err, "Sonarr", instanceId, "stats") {
			return
		}
		return
	}

	h.compareAndLogStatsChanges(instanceId, &result.Stats)

	publishInternalServiceUpdate(h.bc, buildSonarrStatsServiceUpdate(instanceId, &result.Stats, result.Version))

	c.JSON(http.StatusOK, gin.H{
		"stats":   result.Stats,
		"version": result.Version,
	})
}

func (h *SonarrHandler) fetchStats(ctx context.Context, instanceId string) (sonarrStatsResult, error) {
	sonarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "sonarr")
	if err != nil {
		return sonarrStatsResult{}, err
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

	_, episodeCount, _ := summarizeSonarrQueue(records)

	return sonarrStatsResult{
		Stats: types.SonarrStatsResponse{
			QueuedCount:  len(records),
			EpisodeCount: episodeCount,
		},
		Version: version,
	}, nil
}

func (h *SonarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "sonarr", "Sonarr")
	if !ok {
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		log.Error().Msg("No queue ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue ID is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.SonarrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceId, queueId, options)
	})

	if handleQueueDeleteError(c, err, "Sonarr", instanceId, queueId) {
		return
	}

	cacheKey := sonarrQueuePrefix + instanceId
	refreshQueueAfterDelete(
		ctx,
		h.cache,
		h.circuitBreaker,
		cacheKey,
		middleware.CacheDurations.SonarrStatus,
		sonarrStaleDataDuration,
		func() (types.SonarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceId)
		},
		func(result *types.SonarrQueueResponse) {
			publishInternalServiceUpdate(h.bc, buildSonarrQueueServiceUpdate(instanceId, result))
		},
		"Sonarr",
		instanceId,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *SonarrHandler) deleteQueueItem(ctx context.Context, instanceId, queueId string, options types.SonarrQueueDeleteOptions) error {
	sonarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "sonarr")
	if err != nil {
		return err
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(ctx, sonarrConfig.URL, sonarrConfig.APIKey, queueId, options)
}

// Helper methods for change detection
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
