// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"github.com/autobrr/dashbrr/internal/types"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
)

const (
	autobrrStatsCacheDuration = 10 * time.Second
	autobrrIRCCacheDuration   = 5 * time.Second
	statsPrefix               = "autobrr:stats:"
	ircPrefix                 = "autobrr:irc:"
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

func (h *AutobrrHandler) GetAutobrrReleaseStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
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
	stats, err = h.fetchAndCacheStats(instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			log.Debug().
				Str("instanceId", instanceId).
				Msg("Service not configured, returning empty stats")
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

	c.JSON(http.StatusOK, stats)
}

func (h *AutobrrHandler) GetAutobrrIRCStatus(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instance ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Instance ID is required"})
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
	status, err = h.fetchAndCacheIRC(instanceId, cacheKey)
	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
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

	c.JSON(http.StatusOK, status)
}

func (h *AutobrrHandler) fetchAndCacheStats(instanceId, cacheKey string) (autobrr.AutobrrStats, error) {
	autobrrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return autobrr.AutobrrStats{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return autobrr.AutobrrStats{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	stats, err := service.GetReleaseStats(autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return autobrr.AutobrrStats{}, err
	}

	// Cache the results
	ctx := context.Background()
	if err := h.store.Set(ctx, cacheKey, stats, autobrrStatsCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Autobrr release stats")
	}

	return stats, nil
}

func (h *AutobrrHandler) fetchAndCacheIRC(instanceId, cacheKey string) ([]autobrr.IRCStatus, error) {
	autobrrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	status, err := service.GetIRCStatus(autobrrConfig.URL, autobrrConfig.APIKey)
	if err != nil {
		return nil, err
	}

	// Cache the results
	ctx := context.Background()
	if err := h.store.Set(ctx, cacheKey, status, autobrrIRCCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Autobrr IRC status")
	}

	return status, nil
}

func (h *AutobrrHandler) refreshStatsCache(instanceId, cacheKey string) {
	_, err := h.fetchAndCacheStats(instanceId, cacheKey)
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
}

func (h *AutobrrHandler) refreshIRCCache(instanceId, cacheKey string) {
	_, err := h.fetchAndCacheIRC(instanceId, cacheKey)
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
}
