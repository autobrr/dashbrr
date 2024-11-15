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
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	autobrrStatsCacheDuration    = 10 * time.Second
	autobrrIRCCacheDuration      = 5 * time.Second
	autobrrReleasesCacheDuration = 30 * time.Second
	statsPrefix                  = "autobrr:stats:"
	ircPrefix                    = "autobrr:irc:"
	releasesPrefix               = "autobrr:releases:"
)

type AutobrrHandler struct {
	db    *database.DB
	store cache.Store
	sf    *singleflight.Group

	lastReleasesHash  map[string]string
	lastStatsHash     map[string]string
	lastIRCStatusHash map[string]string
	hashMu            sync.Mutex
}

func NewAutobrrHandler(db *database.DB, store cache.Store) *AutobrrHandler {
	return &AutobrrHandler{
		db:    db,
		store: store,
		sf:    &singleflight.Group{},

		// Initialize the new maps
		lastReleasesHash:  make(map[string]string),
		lastStatsHash:     make(map[string]string),
		lastIRCStatusHash: make(map[string]string),
	}
}

func (h *AutobrrHandler) GetAutobrrReleases(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	if instanceId[:7] != "autobrr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	//log.Debug().
	//	Str("instanceId", instanceId).
	//	Msg("GetAutobrrReleases called")

	cacheKey := releasesPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var releases types.ReleasesResponse
	err := h.store.Get(ctx, cacheKey, &releases)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Serving Autobrr releases from cache")
		c.JSON(http.StatusOK, releases)

		// Refresh cache in background without delay
		go h.refreshReleasesCache(instanceId, cacheKey)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("releases:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheReleases(ctx, instanceId, cacheKey)
	})

	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, types.ReleasesResponse{})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("[Autobrr] Request timeout while fetching releases")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("[Autobrr] Failed to fetch releases")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	releases = result.(types.ReleasesResponse)

	h.hashMu.Lock()
	currentHash := createAutobrrReleaseHash(releases)
	lastHash := h.lastReleasesHash[instanceId]

	// Only log when there are releases and the hash has changed
	if (lastHash == "" || currentHash != lastHash) && len(releases.Data) > 0 {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Autobrr] Successfully refreshed releases cache")
	}

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Autobrr releases changed")
		h.lastReleasesHash[instanceId] = currentHash
	}
	h.hashMu.Unlock()

	// Broadcast releases update via SSE
	h.broadcastReleases(instanceId, releases)

	c.JSON(http.StatusOK, releases)
}

func (h *AutobrrHandler) GetAutobrrReleaseStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	if instanceId[:7] != "autobrr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	//log.Debug().
	//	Str("instanceId", instanceId).
	//	Msg("GetAutobrrReleaseStats called")

	cacheKey := statsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var stats types.AutobrrStats
	err := h.store.Get(ctx, cacheKey, &stats)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Interface("stats", stats).
			Msg("Serving Autobrr release stats from cache")

		c.JSON(http.StatusOK, stats)

		// Refresh cache in background without delay
		go h.refreshStatsCache(instanceId, cacheKey)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("stats:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheStats(ctx, instanceId, cacheKey)
	})

	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, types.AutobrrStats{})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Autobrr stats")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Autobrr stats")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	stats = result.(types.AutobrrStats)

	h.hashMu.Lock()
	currentHash := createAutobrrStatsHash(stats)
	lastHash := h.lastStatsHash[instanceId]

	// Only log when there are stats and the hash has changed
	if (lastHash == "" || currentHash != lastHash) && stats.TotalCount > 0 {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Autobrr] Successfully refreshed release stats cache")
	}

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Autobrr] Stats updated")
		h.lastStatsHash[instanceId] = currentHash
	}
	h.hashMu.Unlock()

	// Broadcast stats update via SSE
	h.broadcastStats(instanceId, stats)

	c.JSON(http.StatusOK, stats)
}

func (h *AutobrrHandler) GetAutobrrIRCStatus(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
		return
	}

	if instanceId[:7] != "autobrr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := ircPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var status []types.IRCStatus
	err := h.store.Get(ctx, cacheKey, &status)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Serving Autobrr IRC status from cache")
		c.JSON(http.StatusOK, status)

		// Refresh cache in background without delay
		go h.refreshIRCCache(instanceId, cacheKey)
		return
	}

	// Use singleflight to deduplicate concurrent requests
	sfKey := fmt.Sprintf("irc:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheIRC(ctx, instanceId, cacheKey)
	})

	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, []types.IRCStatus{})
			return
		}

		httpStatus := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			httpStatus = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Autobrr IRC status")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Autobrr IRC status")
		}
		c.JSON(httpStatus, gin.H{"error": err.Error()})
		return
	}

	status = result.([]types.IRCStatus)

	// Broadcast IRC status update via SSE
	h.broadcastIRCStatus(instanceId, status)

	h.hashMu.Lock()
	currentHash := createIRCStatusHash(status)
	lastHash := h.lastIRCStatusHash[instanceId]

	// Only log when there are status entries and the hash has changed
	if (lastHash == "" || currentHash != lastHash) && len(status) > 0 {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("[Autobrr] Successfully refreshed IRC status cache")
	}

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Autobrr IRC status changed")
		h.lastIRCStatusHash[instanceId] = currentHash
	}
	h.hashMu.Unlock()

	c.JSON(http.StatusOK, status)
}

// broadcastReleases broadcasts release updates to all connected SSE clients
func (h *AutobrrHandler) broadcastReleases(instanceId string, releases types.ReleasesResponse) {
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_releases",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": releases,
		},
	})
}

// broadcastStats broadcasts stats updates to all connected SSE clients
func (h *AutobrrHandler) broadcastStats(instanceId string, stats types.AutobrrStats) {
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_stats",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": stats,
		},
	})
}

// broadcastIRCStatus broadcasts IRC status updates to all connected SSE clients
func (h *AutobrrHandler) broadcastIRCStatus(instanceId string, status []types.IRCStatus) {
	// Check for unhealthy IRC connections
	serviceStatus := "online"
	message := "autobrr_irc_status"

	for _, s := range status {
		if !s.Healthy && s.Enabled {
			serviceStatus = "warning"
			message = fmt.Sprintf("IRC network %s is unhealthy", s.Name)
			break
		}
	}

	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      serviceStatus,
		Message:     message,
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"autobrr": map[string]interface{}{
				"irc": status,
			},
		},
	})
}

func (h *AutobrrHandler) fetchAndCacheStats(ctx context.Context, instanceId, cacheKey string) (types.AutobrrStats, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.AutobrrStats{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.AutobrrStats{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	stats, err := service.GetReleaseStats(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return types.AutobrrStats{}, err
	}

	// Cache the results using the centralized cache duration
	if err := h.store.Set(ctx, cacheKey, stats, middleware.CacheDurations.AutobrrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Autobrr release stats")
	}

	return stats, nil
}

func (h *AutobrrHandler) fetchAndCacheReleases(ctx context.Context, instanceId, cacheKey string) (types.ReleasesResponse, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.ReleasesResponse{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.ReleasesResponse{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	releases, err := service.GetReleases(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return types.ReleasesResponse{}, err
	}

	// Cache the results using the centralized cache duration
	if err := h.store.Set(ctx, cacheKey, releases, middleware.CacheDurations.AutobrrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Autobrr releases")
	}

	return releases, nil
}

func (h *AutobrrHandler) fetchAndCacheIRC(ctx context.Context, instanceId, cacheKey string) ([]types.IRCStatus, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	status, err := service.GetIRCStatus(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return nil, err
	}

	// Cache the results using the centralized cache duration
	if err := h.store.Set(ctx, cacheKey, status, middleware.CacheDurations.AutobrrStatus); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Autobrr] Failed to cache IRC status")
	}

	return status, nil
}

func (h *AutobrrHandler) refreshStatsCache(instanceId, cacheKey string) {
	sfKey := fmt.Sprintf("stats_refresh:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		ctx := context.Background()
		return h.fetchAndCacheStats(ctx, instanceId, cacheKey)
	})

	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Autobrr] Failed to refresh release stats cache")
		return
	}

	if err == nil {
		stats := result.(types.AutobrrStats)

		h.hashMu.Lock()
		currentHash := createAutobrrStatsHash(stats)
		lastHash := h.lastStatsHash[instanceId]

		// Only log when there are stats and the hash has changed
		if (lastHash == "" || currentHash != lastHash) && stats.TotalCount > 0 {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("[Autobrr] Successfully refreshed release stats cache")
		}

		if currentHash != lastHash {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("[Autobrr] Stats updated")
			h.lastStatsHash[instanceId] = currentHash
		}
		h.hashMu.Unlock()

		// Broadcast stats update via SSE
		h.broadcastStats(instanceId, stats)
	}
}

func (h *AutobrrHandler) refreshIRCCache(instanceId, cacheKey string) {
	sfKey := fmt.Sprintf("irc_refresh:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		ctx := context.Background()
		return h.fetchAndCacheIRC(ctx, instanceId, cacheKey)
	})

	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Autobrr] Failed to refresh IRC status cache")
		return
	}

	if err == nil {
		status := result.([]types.IRCStatus)

		h.hashMu.Lock()
		currentHash := createIRCStatusHash(status)
		lastHash := h.lastIRCStatusHash[instanceId]

		if (lastHash == "" || currentHash != lastHash) && len(status) > 0 {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("[Autobrr] Successfully refreshed IRC status cache")
		}

		if currentHash != lastHash {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("Autobrr IRC status changed")
			h.lastIRCStatusHash[instanceId] = currentHash
		}
		h.hashMu.Unlock()

		// Broadcast IRC status update via SSE
		h.broadcastIRCStatus(instanceId, status)
	}
}

func (h *AutobrrHandler) refreshReleasesCache(instanceId, cacheKey string) {
	sfKey := fmt.Sprintf("releases_refresh:%s", instanceId)
	result, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		ctx := context.Background()
		return h.fetchAndCacheReleases(ctx, instanceId, cacheKey)
	})

	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Autobrr] Failed to refresh releases cache")
		return
	}

	if err == nil {
		releases := result.(types.ReleasesResponse)

		h.hashMu.Lock()
		currentHash := createAutobrrReleaseHash(releases)
		lastHash := h.lastReleasesHash[instanceId]

		if (lastHash == "" || currentHash != lastHash) && len(releases.Data) > 0 {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("[Autobrr] Successfully refreshed releases cache")
		}

		if currentHash != lastHash {
			log.Debug().
				Str("instanceId", instanceId).
				Msg("Autobrr releases changed")
			h.lastReleasesHash[instanceId] = currentHash
		}
		h.hashMu.Unlock()

		// Broadcast releases update via SSE
		h.broadcastReleases(instanceId, releases)
	}
}

// createAutobrrReleaseHash generates a unique hash representing the current state of Autobrr releases
// The hash includes key release details like title, protocol, and filter status
// This allows for efficient detection of release changes without deep comparison
func createAutobrrReleaseHash(releases types.ReleasesResponse) string {
	if len(releases.Data) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, release := range releases.Data {
		fmt.Fprintf(&sb, "%s:%s:%s,",
			release.Title,
			release.Protocol,
			release.FilterStatus)
	}
	return sb.String()
}

// createAutobrrStatsHash generates a hash representing the current Autobrr statistics
// The hash includes total counts, filtered, rejected, and push-related statistics
// Useful for detecting changes in overall release processing statistics
func createAutobrrStatsHash(stats types.AutobrrStats) string {
	return fmt.Sprintf("%d:%d:%d:%d:%d",
		stats.TotalCount,
		stats.FilteredCount,
		stats.FilterRejectedCount,
		stats.PushApprovedCount,
		stats.PushRejectedCount)
}

// createIRCStatusHash generates a unique hash representing the current IRC connection statuses
// The hash includes the name, health status, and enabled state of each IRC connection
// Helps in detecting changes in IRC connection states efficiently
func createIRCStatusHash(status []types.IRCStatus) string {
	if len(status) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, s := range status {
		fmt.Fprintf(&sb, "%s:%v:%v,", s.Name, s.Healthy, s.Enabled)
	}
	return sb.String()
}
