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

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	prowlarrCacheDuration      = 2 * time.Second // Updated to 2s to match other services
	prowlarrStatsPrefix        = "prowlarr:stats:"
	prowlarrIndexerPrefix      = "prowlarr:indexers:"
	prowlarrIndexerStatsPrefix = "prowlarr:indexerstats:"
)

type ProwlarrHandler struct {
	db                   *database.DB
	cache                cache.Store
	sf                   *singleflight.Group
	lastStatsHash        map[string]string
	lastIndexersHash     map[string]string
	lastIndexerStatsHash map[string]string
	mu                   sync.Mutex
}

func NewProwlarrHandler(db *database.DB, cache cache.Store) *ProwlarrHandler {
	return &ProwlarrHandler{
		db:                   db,
		cache:                cache,
		sf:                   &singleflight.Group{},
		lastStatsHash:        make(map[string]string),
		lastIndexersHash:     make(map[string]string),
		lastIndexerStatsHash: make(map[string]string),
	}
}

func (h *ProwlarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Prowlarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Prowlarr instance
	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Prowlarr] Invalid instance ID")
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
			Msg("[Prowlarr] Serving stats from cache")
		c.JSON(http.StatusOK, statsResp)

		// Broadcast stats update via SSE
		h.broadcastStats(instanceId, statsResp)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("stats:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		prowlarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
		if err != nil {
			return nil, fmt.Errorf("[Prowlarr] failed to get configuration: %w", err)
		}

		if prowlarrConfig == nil {
			return nil, fmt.Errorf("[Prowlarr] is not configured")
		}

		// Build Prowlarr API URL
		apiURL := fmt.Sprintf("%s/api/v1/system/status?apikey=%s", prowlarrConfig.URL, prowlarrConfig.APIKey)

		// Make request to Prowlarr
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("[Prowlarr] failed to fetch stats: %w", err)
		}

		if resp == nil {
			return nil, fmt.Errorf("[Prowlarr] received nil response")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("[Prowlarr] API returned status: %d", resp.StatusCode)
		}

		var stats types.ProwlarrStatsResponse
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return nil, fmt.Errorf("[Prowlarr] failed to parse response: %w", err)
		}

		return stats, nil
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

	statsResp = result.(types.ProwlarrStatsResponse)

	// Add hash-based change detection
	h.mu.Lock()
	currentHash := createStatsHash(statsResp)
	lastHash := h.lastStatsHash[instanceId]

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Int("grabCount", statsResp.GrabCount).
			Str("change", "stats_changed").
			Msg("[Prowlarr] Successfully retrieved and cached stats")

		h.lastStatsHash[instanceId] = currentHash
	}
	h.mu.Unlock()

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, statsResp, prowlarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Prowlarr] Failed to cache stats")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("grabCount", statsResp.GrabCount).
		Msg("[Prowlarr] Successfully retrieved and cached stats")

	// Broadcast stats update via SSE
	h.broadcastStats(instanceId, statsResp)

	c.JSON(http.StatusOK, statsResp)
}

// Helper method to broadcast stats updates
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

		// Broadcast indexers update via SSE
		h.broadcastIndexers(instanceId, indexers)

		c.JSON(http.StatusOK, indexers)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("indexers:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		prowlarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
		if err != nil {
			return nil, fmt.Errorf("failed to get Prowlarr configuration: %w", err)
		}

		if prowlarrConfig == nil {
			return nil, fmt.Errorf("prowlarr is not configured")
		}

		// Build Prowlarr API URL
		apiURL := fmt.Sprintf("%s/api/v1/indexer?apikey=%s", prowlarrConfig.URL, prowlarrConfig.APIKey)

		// Make request to Prowlarr
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Prowlarr indexers: %w", err)
		}

		if resp == nil {
			return nil, fmt.Errorf("received nil response from Prowlarr")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("prowlarr API returned status: %d", resp.StatusCode)
		}

		var indexers []types.ProwlarrIndexer
		if err := json.NewDecoder(resp.Body).Decode(&indexers); err != nil {
			return nil, fmt.Errorf("failed to parse Prowlarr response: %w", err)
		}

		// Get indexer stats
		prowlarrService := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
		statsResp, err := prowlarrService.GetIndexerStats(ctx, prowlarrConfig.URL, prowlarrConfig.APIKey)
		if err == nil && statsResp != nil {
			// Create a map for quick lookup
			statsMap := make(map[int]types.ProwlarrIndexerStats)
			for _, stat := range statsResp.Indexers {
				statsMap[stat.IndexerID] = stat
			}

			// Enrich indexers with stats
			for i := range indexers {
				if stats, ok := statsMap[indexers[i].ID]; ok {
					indexers[i].AverageResponseTime = stats.AverageResponseTime
					indexers[i].NumberOfGrabs = stats.NumberOfGrabs
					indexers[i].NumberOfQueries = stats.NumberOfQueries
				}
			}
		}

		return indexers, nil
	})

	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Prowlarr indexers")
		status := http.StatusInternalServerError
		if err.Error() == "prowlarr is not configured" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	indexers = result.([]types.ProwlarrIndexer)

	// Add hash-based change detection
	h.mu.Lock()
	currentHash := createIndexersHash(indexers)
	lastHash := h.lastIndexersHash[instanceId]

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(indexers)).
			Str("change", "indexers_changed").
			Msg("[Prowlarr] Successfully retrieved and cached indexers")

		h.lastIndexersHash[instanceId] = currentHash
	}
	h.mu.Unlock()

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, indexers, prowlarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Prowlarr] Failed to cache indexers")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("indexerCount", len(indexers)).
		Msg("[Prowlarr] Successfully retrieved and cached indexers")

	// Broadcast indexers update via SSE
	h.broadcastIndexers(instanceId, indexers)

	c.JSON(http.StatusOK, indexers)
}

// Helper method to broadcast indexers updates
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

func (h *ProwlarrHandler) GetIndexerStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("[Prowlarr] No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Prowlarr instance
	if instanceId[:8] != "prowlarr" {
		log.Error().Str("instanceId", instanceId).Msg("[Prowlarr] Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prowlarr instance ID"})
		return
	}

	cacheKey := prowlarrIndexerStatsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var statsResp types.ProwlarrIndexerStatsResponse
	err := h.cache.Get(ctx, cacheKey, &statsResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(statsResp.Indexers)).
			Msg("[Prowlarr] Serving indexer stats from cache")
		c.JSON(http.StatusOK, statsResp)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("indexer_stats:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		prowlarrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
		if err != nil {
			return nil, fmt.Errorf("failed to get Prowlarr configuration: %w", err)
		}

		if prowlarrConfig == nil {
			return nil, fmt.Errorf("prowlarr is not configured")
		}

		// Get indexer stats
		prowlarrService := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
		stats, err := prowlarrService.GetIndexerStats(ctx, prowlarrConfig.URL, prowlarrConfig.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Prowlarr indexer stats: %w", err)
		}

		return stats, nil
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

	statsResp = result.(types.ProwlarrIndexerStatsResponse)

	// Add hash-based change detection
	h.mu.Lock()
	currentHash := createIndexerStatsHash(statsResp)
	lastHash := h.lastIndexerStatsHash[instanceId]

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Int("indexerCount", len(statsResp.Indexers)).
			Str("change", "indexer_stats_changed").
			Msg("[Prowlarr] Successfully retrieved and cached indexer stats")

		h.lastIndexerStatsHash[instanceId] = currentHash
	}
	h.mu.Unlock()

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, statsResp, prowlarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Prowlarr] Failed to cache indexer stats")
	}

	c.JSON(http.StatusOK, statsResp)
}

// createStatsHash generates a unique hash for Prowlarr system stats
// The purpose is to efficiently detect meaningful changes in system performance
// By comparing grab and fail counts, we can avoid unnecessary logging and updates
// when the system state remains fundamentally unchanged
func createStatsHash(stats types.ProwlarrStatsResponse) string {
	return fmt.Sprintf("%d:%d", stats.GrabCount, stats.FailCount)
}

// createIndexersHash generates a unique hash representing the current state of Prowlarr indexers
// This method helps reduce unnecessary processing and logging by detecting actual changes
// It captures key indexer characteristics like ID, name, and number of grabs
// Allows for quick comparison without deep object traversal, minimizing performance overhead
func createIndexersHash(indexers []types.ProwlarrIndexer) string {
	var sb strings.Builder
	for _, indexer := range indexers {
		fmt.Fprintf(&sb, "%d:%s:%d,",
			indexer.ID,
			indexer.Name,
			indexer.NumberOfGrabs)
	}
	return sb.String()
}

// createIndexerStatsHash generates a unique hash for Prowlarr indexer statistics
// Designed to efficiently track meaningful changes in indexer performance
// 1. Prevent redundant logging and processing
// 2. Reduce unnecessary system updates when no substantial changes occur
func createIndexerStatsHash(stats types.ProwlarrIndexerStatsResponse) string {
	var sb strings.Builder
	for _, indexerStat := range stats.Indexers {
		fmt.Fprintf(&sb, "%d:%d:%d,",
			indexerStat.IndexerID,
			indexerStat.NumberOfQueries,
			indexerStat.NumberOfGrabs)
	}
	return sb.String()
}
