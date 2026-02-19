// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	prowlarrStatsPrefix        = "prowlarr:stats:"
	prowlarrIndexerPrefix      = "prowlarr:indexers:"
	prowlarrIndexerStatsPrefix = "prowlarr:indexerstats:"
	prowlarrStaleDataDuration  = 5 * time.Minute // How long to serve stale data
)

type ProwlarrHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	// Single hash map and mutex for all state tracking
	lastHash   map[string]string // key format: "stats:instanceId", "indexers:instanceId", etc.
	lastHashMu sync.Mutex
}

func NewProwlarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *ProwlarrHandler {
	return &ProwlarrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastHash:       make(map[string]string),
	}
}

// fetchProwlarrData handles fetching all required data in parallel.
func (h *ProwlarrHandler) fetchProwlarrData(ctx context.Context, instanceId string) (types.ProwlarrStatsResponse, []types.ProwlarrIndexer, types.ProwlarrIndexerStatsResponse, error) {
	prowlarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "prowlarr")
	if err != nil {
		return types.ProwlarrStatsResponse{}, nil, types.ProwlarrIndexerStatsResponse{}, err
	}

	var (
		indexers                     []types.ProwlarrIndexer
		indexerStats                 types.ProwlarrIndexerStatsResponse
		indexersErr, indexerStatsErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// Indexers request
	go func() {
		defer wg.Done()

		prowlarrService := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
		i, err := prowlarrService.GetIndexers(ctx, prowlarrConfig.URL, prowlarrConfig.APIKey)

		if err != nil {
			indexersErr = err
			return
		}
		indexers = i
	}()

	// Indexer stats request
	go func() {
		defer wg.Done()

		prowlarrService := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
		stats, err := prowlarrService.GetIndexerStats(ctx, prowlarrConfig.URL, prowlarrConfig.APIKey)
		if err != nil {
			indexerStatsErr = err
			return
		}
		if stats == nil {
			indexerStatsErr = fmt.Errorf("prowlarr indexer stats unavailable")
			return
		}
		indexerStats = *stats
	}()

	wg.Wait()

	// Check for errors
	if indexersErr != nil && indexerStatsErr != nil {
		return types.ProwlarrStatsResponse{}, nil, types.ProwlarrIndexerStatsResponse{},
			fmt.Errorf("all requests failed: indexers: %v, indexer stats: %v",
				indexersErr, indexerStatsErr)
	}

	// Derive stats, enrich indexers when both payloads are available.
	stats := types.ProwlarrStatsResponse{}

	if indexerStatsErr == nil {
		statsMap := make(map[int]types.ProwlarrIndexerStats)
		totalGrabs := 0
		totalFails := 0
		for _, stat := range indexerStats.Indexers {
			statsMap[stat.IndexerID] = stat
			totalGrabs += stat.NumberOfGrabs
			totalFails += stat.NumberOfFailedGrabs
		}

		stats.GrabCount = totalGrabs
		stats.FailCount = totalFails
		if indexersErr == nil {
			stats.IndexerCount = len(indexers)
			for i := range indexers {
				if s, ok := statsMap[indexers[i].ID]; ok {
					indexers[i].AverageResponseTime = s.AverageResponseTime
					indexers[i].NumberOfGrabs = s.NumberOfGrabs
					indexers[i].NumberOfQueries = s.NumberOfQueries
				}
			}
		} else {
			stats.IndexerCount = len(indexerStats.Indexers)
		}
	} else if indexersErr == nil {
		stats = types.ProwlarrStatsResponse{IndexerCount: len(indexers)}
	}

	return stats, indexers, indexerStats, nil
}

func (h *ProwlarrHandler) GetStats(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "prowlarr", "Prowlarr")
	if !ok {
		return
	}

	cacheKey := prowlarrStatsPrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.ProwlarrStatsResponse]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.ProwlarrStatus,
		StaleTTL:       prowlarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() (types.ProwlarrStatsResponse, error) {
			stats, _, _, err := h.fetchProwlarrData(ctx, instanceId)
			return stats, err
		},
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch stats")
		c.JSON(statusFromProwlarrError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogStatsChanges(instanceId, result)
	h.broadcastStats(instanceId, result)

	c.JSON(http.StatusOK, result)
}

func (h *ProwlarrHandler) GetIndexers(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "prowlarr", "Prowlarr")
	if !ok {
		return
	}

	cacheKey := prowlarrIndexerPrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[[]types.ProwlarrIndexer]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.ProwlarrStatus,
		StaleTTL:       prowlarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() ([]types.ProwlarrIndexer, error) {
			_, indexers, _, err := h.fetchProwlarrData(ctx, instanceId)
			if err != nil {
				return nil, err
			}
			if indexers == nil {
				return nil, fmt.Errorf("prowlarr indexers unavailable")
			}
			return indexers, nil
		},
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch indexers")
		c.JSON(statusFromProwlarrError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogIndexersChanges(instanceId, result)
	h.broadcastIndexers(instanceId, result)

	c.JSON(http.StatusOK, result)
}

func (h *ProwlarrHandler) GetIndexerStats(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "prowlarr", "Prowlarr")
	if !ok {
		return
	}

	cacheKey := prowlarrIndexerStatsPrefix + instanceId
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.ProwlarrIndexerStatsResponse]{
		Store:          h.cache,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.ProwlarrStatus,
		StaleTTL:       prowlarrStaleDataDuration,
		CircuitBreaker: h.circuitBreaker,
		Fetch: func() (types.ProwlarrIndexerStatsResponse, error) {
			_, _, stats, err := h.fetchProwlarrData(ctx, instanceId)
			return stats, err
		},
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch indexer stats")
		c.JSON(statusFromProwlarrError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogIndexerStatsChanges(instanceId, result)

	c.JSON(http.StatusOK, result)
}

// Helper methods for change detection
func (h *ProwlarrHandler) createStatsHash(stats types.ProwlarrStatsResponse) string {
	return fmt.Sprintf("%d:%d", stats.GrabCount, stats.FailCount)
}

func (h *ProwlarrHandler) detectStatsChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_stats"
	}
	if oldHash != newHash {
		return "stats_changed"
	}
	return "no_change"
}

func (h *ProwlarrHandler) compareAndLogStatsChanges(instanceId string, stats types.ProwlarrStatsResponse) {
	h.lastHashMu.Lock()
	defer h.lastHashMu.Unlock()

	key := fmt.Sprintf("stats:%s", instanceId)
	currentHash := h.createStatsHash(stats)
	lastHash := h.lastHash[key]

	if currentHash != lastHash {
		changes := h.detectStatsChanges(lastHash, currentHash)
		log.Debug().
			Str("instanceId", instanceId).
			Int("grabCount", stats.GrabCount).
			Str("change", changes).
			Msg("[Prowlarr] Stats changed")

		h.lastHash[key] = currentHash
	}
}

func (h *ProwlarrHandler) createIndexersHash(indexers []types.ProwlarrIndexer) string {
	var sb strings.Builder
	for _, indexer := range indexers {
		fmt.Fprintf(&sb, "%d:%s:%d,",
			indexer.ID,
			indexer.Name,
			indexer.NumberOfGrabs)
	}
	return sb.String()
}

func (h *ProwlarrHandler) detectIndexersChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_indexers"
	}

	oldIndexers := strings.Split(oldHash, ",")
	newIndexers := strings.Split(newHash, ",")

	if len(oldIndexers) < len(newIndexers) {
		return "indexer_added"
	} else if len(oldIndexers) > len(newIndexers) {
		return "indexer_removed"
	}

	return "indexer_updated"
}

func (h *ProwlarrHandler) compareAndLogIndexersChanges(instanceId string, indexers []types.ProwlarrIndexer) {
	h.lastHashMu.Lock()
	defer h.lastHashMu.Unlock()

	key := fmt.Sprintf("indexers:%s", instanceId)
	currentHash := h.createIndexersHash(indexers)
	lastHash := h.lastHash[key]

	if currentHash != lastHash {
		changes := h.detectIndexersChanges(lastHash, currentHash)
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(indexers)).
			Str("change", changes).
			Msg("[Prowlarr] Indexers changed")

		h.lastHash[key] = currentHash
	}
}

func (h *ProwlarrHandler) createIndexerStatsHash(stats types.ProwlarrIndexerStatsResponse) string {
	var sb strings.Builder
	for _, indexerStat := range stats.Indexers {
		fmt.Fprintf(&sb, "%d:%d:%d,",
			indexerStat.IndexerID,
			indexerStat.NumberOfQueries,
			indexerStat.NumberOfGrabs)
	}
	return sb.String()
}

func (h *ProwlarrHandler) detectIndexerStatsChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_stats"
	}
	if oldHash != newHash {
		return "stats_changed"
	}
	return "no_change"
}

func (h *ProwlarrHandler) compareAndLogIndexerStatsChanges(instanceId string, stats types.ProwlarrIndexerStatsResponse) {
	h.lastHashMu.Lock()
	defer h.lastHashMu.Unlock()

	key := fmt.Sprintf("indexer_stats:%s", instanceId)
	currentHash := h.createIndexerStatsHash(stats)
	lastHash := h.lastHash[key]

	if currentHash != lastHash {
		changes := h.detectIndexerStatsChanges(lastHash, currentHash)
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(stats.Indexers)).
			Str("change", changes).
			Msg("[Prowlarr] Indexer stats changed")

		h.lastHash[key] = currentHash
	}
}

func (h *ProwlarrHandler) broadcastStats(instanceId string, stats types.ProwlarrStatsResponse) {
	publishInternalServiceUpdate(h.bc, buildProwlarrStatsServiceUpdate(instanceId, stats))
}

func (h *ProwlarrHandler) broadcastIndexers(instanceId string, indexers []types.ProwlarrIndexer) {
	publishInternalServiceUpdate(h.bc, buildProwlarrIndexersServiceUpdate(instanceId, indexers))
}

func statusFromProwlarrError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var arrErr *arr.ErrArr
	if errors.As(err, &arrErr) && arrErr.HttpCode > 0 {
		return normalizeUpstreamStatus(arrErr.HttpCode)
	}

	return http.StatusInternalServerError
}
