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
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	autobrrStatsCacheDuration    = 10 * time.Second
	autobrrIRCCacheDuration      = 5 * time.Second
	autobrrReleasesCacheDuration = 30 * time.Second
	autobrrStaleDataDuration     = 5 * time.Minute
	backgroundTimeout            = 5 * time.Second
	statsPrefix                  = "autobrr:stats:"
	ircPrefix                    = "autobrr:irc:"
	releasesPrefix               = "autobrr:releases:"
)

type AutobrrHandler struct {
	db             *database.DB
	store          cache.Store
	bc             *Broadcaster
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastReleasesHash  map[string]string
	lastStatsHash     map[string]string
	lastIRCStatusHash map[string]string
	hashMu            sync.Mutex
}

func NewAutobrrHandler(db *database.DB, store cache.Store, bc *Broadcaster) *AutobrrHandler {
	return &AutobrrHandler{
		db:             db,
		store:          store,
		bc:             bc,
		sf:             &singleflight.Group{},
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute),

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

	if !strings.HasPrefix(instanceId, "autobrr") {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := releasesPrefix + instanceId
	ctx := c.Request.Context() // Use request context instead of background

	releases, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.ReleasesResponse]{
		Store:           h.store,
		Key:             cacheKey,
		FreshTTL:        middleware.CacheDurations.AutobrrStatus,
		StaleTTL:        autobrrStaleDataDuration,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    h.sf,
		SingleflightKey: fmt.Sprintf("releases:%s", instanceId),
		Fetch: func() (types.ReleasesResponse, error) {
			return h.fetchReleases(ctx, instanceId)
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
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

	h.hashMu.Lock()
	currentHash := createAutobrrReleaseHash(releases)
	lastHash := h.lastReleasesHash[instanceId]

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

	if !strings.HasPrefix(instanceId, "autobrr") {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := statsPrefix + instanceId
	ctx := c.Request.Context() // Use request context instead of background

	stats, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.AutobrrStats]{
		Store:           h.store,
		Key:             cacheKey,
		FreshTTL:        middleware.CacheDurations.AutobrrStatus,
		StaleTTL:        autobrrStaleDataDuration,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    h.sf,
		SingleflightKey: fmt.Sprintf("stats:%s", instanceId),
		Fetch: func() (types.AutobrrStats, error) {
			return h.fetchStats(ctx, instanceId)
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
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

	h.hashMu.Lock()
	currentHash := createAutobrrStatsHash(stats)
	lastHash := h.lastStatsHash[instanceId]

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

	if !strings.HasPrefix(instanceId, "autobrr") {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := ircPrefix + instanceId
	ctx := c.Request.Context() // Use request context instead of background

	status, err := FetchWithSWRCache(ctx, SWRCacheOptions[[]types.IRCStatus]{
		Store:           h.store,
		Key:             cacheKey,
		FreshTTL:        middleware.CacheDurations.AutobrrStatus,
		StaleTTL:        autobrrStaleDataDuration,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    h.sf,
		SingleflightKey: fmt.Sprintf("irc:%s", instanceId),
		Fetch: func() ([]types.IRCStatus, error) {
			status, err := h.fetchIRC(ctx, instanceId)
			if err != nil {
				return nil, err
			}
			if status == nil {
				status = make([]types.IRCStatus, 0)
			}
			return status, nil
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
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

	h.hashMu.Lock()
	currentHash := createIRCStatusHash(status)
	lastHash := h.lastIRCStatusHash[instanceId]

	if currentHash != lastHash {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Autobrr IRC status changed")
		h.lastIRCStatusHash[instanceId] = currentHash
	}
	h.hashMu.Unlock()

	// Broadcast IRC status update via SSE
	h.broadcastIRCStatus(instanceId, status)

	c.JSON(http.StatusOK, status)
}

func (h *AutobrrHandler) fetchStats(ctx context.Context, instanceId string) (types.AutobrrStats, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.AutobrrStats{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.AutobrrStats{}, ErrServiceNotConfigured
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	return service.GetReleaseStats(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
}

func (h *AutobrrHandler) fetchReleases(ctx context.Context, instanceId string) (types.ReleasesResponse, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.ReleasesResponse{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.ReleasesResponse{}, ErrServiceNotConfigured
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	return service.GetReleases(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
}

func (h *AutobrrHandler) fetchIRC(ctx context.Context, instanceId string) ([]types.IRCStatus, error) {
	autobrrConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return nil, ErrServiceNotConfigured
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	return service.GetIRCStatus(ctx, autobrrConfig.URL, autobrrConfig.APIKey)
}

// broadcastReleases broadcasts release updates to all connected SSE clients
func (h *AutobrrHandler) broadcastReleases(instanceId string, releases types.ReleasesResponse) {
	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_releases",
		EventType:   models.ServiceEventInternal,
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": map[string]interface{}{
				"releases": releases,
			},
		},
	}

	h.bc.Publish(health)
}

// broadcastStats broadcasts stats updates to all connected SSE clients
func (h *AutobrrHandler) broadcastStats(instanceId string, stats types.AutobrrStats) {
	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_stats",
		EventType:   models.ServiceEventInternal,
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": map[string]interface{}{
				"stats": stats,
			},
		},
	}

	h.bc.Publish(health)
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

	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      serviceStatus,
		Message:     message,
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"autobrr": types.AutobrrDetails{
				IRC: status,
			},
		},
	}
	if message == "autobrr_irc_status" {
		health.EventType = models.ServiceEventInternal
	}

	h.bc.Publish(health)
}

// Hash generation functions
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

func createAutobrrStatsHash(stats types.AutobrrStats) string {
	return fmt.Sprintf("%d:%d:%d:%d:%d",
		stats.TotalCount,
		stats.FilteredCount,
		stats.FilterRejectedCount,
		stats.PushApprovedCount,
		stats.PushRejectedCount)
}

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
