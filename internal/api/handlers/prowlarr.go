// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
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
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	// Single hash map and mutex for all state tracking
	lastHash   map[string]string // key format: "stats:instanceId", "indexers:instanceId", etc.
	lastHashMu sync.Mutex
}

func NewProwlarrHandler(db *database.DB, cache cache.Store) *ProwlarrHandler {
	return &ProwlarrHandler{
		db:             db,
		cache:          cache,
		sf:             &singleflight.Group{},
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastHash:       make(map[string]string),
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *ProwlarrHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.ProwlarrStatus + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					_ = h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.ProwlarrStatus)
				}
			}
		}()
		return data, nil
	}

	// Check circuit breaker before making request
	if h.circuitBreaker.IsOpen() {
		// Try to get stale data when circuit is open
		var staleData interface{}
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// Cache miss or error, fetch fresh data with retry
	var fetchErr error
	err = resilience.RetryWithBackoff(ctx, func() error {
		data, fetchErr = fetchFn()
		return fetchErr
	})

	if err != nil {
		h.circuitBreaker.RecordFailure()
		// Try to get stale data
		var staleData interface{}
		if staleErr := h.cache.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return nil, err
	}

	h.circuitBreaker.RecordSuccess()

	// Cache the fresh data
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.ProwlarrStatus); err == nil {
		// Also cache as stale data with longer duration
		_ = h.cache.Set(ctx, cacheKey+":stale", data, prowlarrStaleDataDuration)
	}

	return data, nil
}

// fetchProwlarrData handles fetching all required data in parallel
func (h *ProwlarrHandler) fetchProwlarrData(ctx context.Context, instanceId string) (types.ProwlarrStatsResponse, []types.ProwlarrIndexer, types.ProwlarrIndexerStatsResponse, error) {
	prowlarrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.ProwlarrStatsResponse{}, nil, types.ProwlarrIndexerStatsResponse{}, fmt.Errorf("failed to get configuration: %w", err)
	}

	if prowlarrConfig == nil {
		return types.ProwlarrStatsResponse{}, nil, types.ProwlarrIndexerStatsResponse{}, fmt.Errorf("prowlarr is not configured")
	}

	var (
		stats                                  types.ProwlarrStatsResponse
		indexers                               []types.ProwlarrIndexer
		indexerStats                           types.ProwlarrIndexerStatsResponse
		statsErr, indexersErr, indexerStatsErr error
	)

	// Create request functions for concurrent execution
	requests := []func() (interface{}, error){
		// Stats request
		func() (interface{}, error) {
			apiURL := fmt.Sprintf("%s/api/v1/system/status", prowlarrConfig.URL)
			resp, err := arr.MakeArrRequest(ctx, http.MethodGet, apiURL, prowlarrConfig.APIKey, nil)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			var s types.ProwlarrStatsResponse
			if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
				return nil, err
			}
			return s, nil
		},
		// Indexers request
		func() (interface{}, error) {
			apiURL := fmt.Sprintf("%s/api/v1/indexer", prowlarrConfig.URL)
			resp, err := arr.MakeArrRequest(ctx, http.MethodGet, apiURL, prowlarrConfig.APIKey, nil)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			var i []types.ProwlarrIndexer
			if err := json.NewDecoder(resp.Body).Decode(&i); err != nil {
				return nil, err
			}
			return i, nil
		},
		// Indexer stats request
		func() (interface{}, error) {
			prowlarrService := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
			return prowlarrService.GetIndexerStats(ctx, prowlarrConfig.URL, prowlarrConfig.APIKey)
		},
	}

	// Execute requests concurrently
	results := make(chan struct {
		index  int
		result interface{}
		err    error
	}, len(requests))

	for i, req := range requests {
		go func(index int, request func() (interface{}, error)) {
			result, err := request()
			results <- struct {
				index  int
				result interface{}
				err    error
			}{index, result, err}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		result := <-results
		switch result.index {
		case 0:
			if result.err != nil {
				statsErr = result.err
			} else {
				stats = result.result.(types.ProwlarrStatsResponse)
			}
		case 1:
			if result.err != nil {
				indexersErr = result.err
			} else {
				indexers = result.result.([]types.ProwlarrIndexer)
			}
		case 2:
			if result.err != nil {
				indexerStatsErr = result.err
			} else {
				indexerStats = *result.result.(*types.ProwlarrIndexerStatsResponse)
			}
		}
	}

	// Check for errors
	if statsErr != nil && indexersErr != nil && indexerStatsErr != nil {
		return types.ProwlarrStatsResponse{}, nil, types.ProwlarrIndexerStatsResponse{},
			fmt.Errorf("all requests failed: stats: %v, indexers: %v, indexer stats: %v",
				statsErr, indexersErr, indexerStatsErr)
	}

	// Enrich indexers with stats if both are available
	if indexerStatsErr == nil && indexersErr == nil {
		statsMap := make(map[int]types.ProwlarrIndexerStats)
		for _, stat := range indexerStats.Indexers {
			statsMap[stat.IndexerID] = stat
		}

		for i := range indexers {
			if stats, ok := statsMap[indexers[i].ID]; ok {
				indexers[i].AverageResponseTime = stats.AverageResponseTime
				indexers[i].NumberOfGrabs = stats.NumberOfGrabs
				indexers[i].NumberOfQueries = stats.NumberOfQueries
			}
		}
	}

	return stats, indexers, indexerStats, nil
}

func (h *ProwlarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Prowlarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Prowlarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrStatsPrefix + instanceId
	ctx := context.Background()

	result, err := h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
		stats, _, _, err := h.fetchProwlarrData(ctx, instanceId)
		return stats, err
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch stats")
		status := http.StatusInternalServerError
		if err.Error() == "prowlarr is not configured" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	statsResp := result.(types.ProwlarrStatsResponse)
	h.compareAndLogStatsChanges(instanceId, statsResp)
	h.broadcastStats(instanceId, statsResp)

	c.JSON(http.StatusOK, statsResp)
}

func (h *ProwlarrHandler) GetIndexers(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Prowlarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Prowlarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrIndexerPrefix + instanceId
	ctx := context.Background()

	result, err := h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
		_, indexers, _, err := h.fetchProwlarrData(ctx, instanceId)
		return indexers, err
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch indexers")
		status := http.StatusInternalServerError
		if err.Error() == "prowlarr is not configured" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	indexers := result.([]types.ProwlarrIndexer)
	h.compareAndLogIndexersChanges(instanceId, indexers)
	h.broadcastIndexers(instanceId, indexers)

	c.JSON(http.StatusOK, indexers)
}

func (h *ProwlarrHandler) GetIndexerStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Prowlarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Prowlarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrIndexerStatsPrefix + instanceId
	ctx := context.Background()

	result, err := h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
		_, _, stats, err := h.fetchProwlarrData(ctx, instanceId)
		return stats, err
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("[Prowlarr] Failed to fetch indexer stats")
		status := http.StatusInternalServerError
		if err.Error() == "prowlarr is not configured" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	statsResp := result.(types.ProwlarrIndexerStatsResponse)
	h.compareAndLogIndexerStatsChanges(instanceId, statsResp)

	c.JSON(http.StatusOK, statsResp)
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
	BroadcastHealth(models.ServiceHealth{
		ServiceID: instanceId,
		Status:    "ok",
		Message:   "prowlarr_stats",
		Stats: map[string]interface{}{
			"prowlarr": map[string]interface{}{
				"stats": stats,
			},
		},
	})
}

func (h *ProwlarrHandler) broadcastIndexers(instanceId string, indexers []types.ProwlarrIndexer) {
	BroadcastHealth(models.ServiceHealth{
		ServiceID: instanceId,
		Status:    "ok",
		Message:   "prowlarr_indexers",
		Stats: map[string]interface{}{
			"prowlarr": map[string]interface{}{
				"indexers": indexers,
			},
		},
	})
}
