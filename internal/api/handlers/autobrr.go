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
}

func NewAutobrrHandler(db *database.DB, store cache.Store) *AutobrrHandler {
	return &AutobrrHandler{
		db:    db,
		store: store,
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

	log.Debug().
		Str("instanceId", instanceId).
		Msg("GetAutobrrReleases called")

	cacheKey := releasesPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var releases autobrr.ReleasesResponse
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

	// If not in cache, fetch from service
	releases, err = h.fetchAndCacheReleases(ctx, instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, autobrr.ReleasesResponse{})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Autobrr releases")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Autobrr releases")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully retrieved and cached Autobrr releases")

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

	log.Debug().
		Str("instanceId", instanceId).
		Msg("GetAutobrrReleaseStats called")

	cacheKey := statsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var stats autobrr.AutobrrStats
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

	// If not in cache, fetch from service
	stats, err = h.fetchAndCacheStats(ctx, instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, autobrr.AutobrrStats{})
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

	log.Debug().
		Str("instanceId", instanceId).
		Interface("stats", stats).
		Msg("Successfully retrieved and cached autobrr release stats")

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
	var status []autobrr.IRCStatus
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

	// If not in cache, fetch from service
	status, err = h.fetchAndCacheIRC(ctx, instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			c.JSON(http.StatusOK, []autobrr.IRCStatus{})
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

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully retrieved and cached Autobrr IRC status")

	// Broadcast IRC status update via SSE
	h.broadcastIRCStatus(instanceId, status)

	c.JSON(http.StatusOK, status)
}

// broadcastReleases broadcasts release updates to all connected SSE clients
func (h *AutobrrHandler) broadcastReleases(instanceId string, releases autobrr.ReleasesResponse) {
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
func (h *AutobrrHandler) broadcastStats(instanceId string, stats autobrr.AutobrrStats) {
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
func (h *AutobrrHandler) broadcastIRCStatus(instanceId string, status []autobrr.IRCStatus) {
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

func (h *AutobrrHandler) fetchAndCacheStats(ctx context.Context, instanceId, cacheKey string) (autobrr.AutobrrStats, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return autobrr.AutobrrStats{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return autobrr.AutobrrStats{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	stats, err := service.GetReleaseStats(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return autobrr.AutobrrStats{}, err
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

func (h *AutobrrHandler) fetchAndCacheReleases(ctx context.Context, instanceId, cacheKey string) (autobrr.ReleasesResponse, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return autobrr.ReleasesResponse{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return autobrr.ReleasesResponse{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	releases, err := service.GetReleases(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return autobrr.ReleasesResponse{}, err
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

func (h *AutobrrHandler) fetchAndCacheIRC(ctx context.Context, instanceId, cacheKey string) ([]autobrr.IRCStatus, error) {
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
			Msg("Failed to cache Autobrr IRC status")
	}

	return status, nil
}

func (h *AutobrrHandler) refreshStatsCache(instanceId, cacheKey string) {
	ctx := context.Background()
	stats, err := h.fetchAndCacheStats(ctx, instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Autobrr release stats cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully refreshed Autobrr release stats cache")

	// Broadcast stats update via SSE
	h.broadcastStats(instanceId, stats)
}

func (h *AutobrrHandler) refreshIRCCache(instanceId, cacheKey string) {
	ctx := context.Background()
	status, err := h.fetchAndCacheIRC(ctx, instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh autobrr IRC status cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully refreshed autobrr IRC status cache")

	// Broadcast IRC status update via SSE
	h.broadcastIRCStatus(instanceId, status)
}

func (h *AutobrrHandler) refreshReleasesCache(instanceId, cacheKey string) {
	ctx := context.Background()
	releases, err := h.fetchAndCacheReleases(ctx, instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh autobrr releases cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully refreshed autobrr releases cache")

	// Broadcast releases update via SSE
	h.broadcastReleases(instanceId, releases)
}
