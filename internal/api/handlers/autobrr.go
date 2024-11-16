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
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/internal/utils"
)

const (
	autobrrStatsCacheDuration    = 10 * time.Second
	autobrrIRCCacheDuration      = 5 * time.Second
	autobrrReleasesCacheDuration = 30 * time.Second
	autobrrStaleDataDuration     = 5 * time.Minute
	statsPrefix                  = "autobrr:stats:"
	ircPrefix                    = "autobrr:irc:"
	releasesPrefix               = "autobrr:releases:"
)

type AutobrrHandler struct {
	db             *database.DB
	store          cache.Store
	sf             *singleflight.Group
	circuitBreaker *resilience.CircuitBreaker

	lastReleasesHash  map[string]string
	lastStatsHash     map[string]string
	lastIRCStatusHash map[string]string
	hashMu            sync.Mutex
}

func NewAutobrrHandler(db *database.DB, store cache.Store) *AutobrrHandler {
	return &AutobrrHandler{
		db:             db,
		store:          store,
		sf:             &singleflight.Group{},
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute),

		lastReleasesHash:  make(map[string]string),
		lastStatsHash:     make(map[string]string),
		lastIRCStatusHash: make(map[string]string),
	}
}

// fetchDataWithCache implements a type-safe stale-while-revalidate pattern
func fetchDataWithCache[T any](ctx context.Context, store cache.Store, circuitBreaker *resilience.CircuitBreaker, cacheKey string, fetchFn func() (T, error)) (T, error) {
	var data T

	// Try to get from cache first
	err := store.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.AutobrrStatus + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					_ = store.Set(ctx, cacheKey, newData, middleware.CacheDurations.AutobrrStatus)
				}
			}
		}()
		return data, nil
	}

	// Check circuit breaker before making request
	if circuitBreaker.IsOpen() {
		// Try to get stale data when circuit is open
		var staleData T
		if staleErr := store.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return data, fmt.Errorf("circuit breaker is open")
	}

	// Cache miss or error, fetch fresh data with retry
	var freshData T
	err = resilience.RetryWithBackoff(ctx, func() error {
		var fetchErr error
		freshData, fetchErr = fetchFn()
		return fetchErr
	})

	if err != nil {
		circuitBreaker.RecordFailure()
		// Try to get stale data
		var staleData T
		if staleErr := store.Get(ctx, cacheKey+":stale", &staleData); staleErr == nil {
			return staleData, nil
		}
		return data, err
	}

	circuitBreaker.RecordSuccess()

	// Cache the fresh data
	if err := store.Set(ctx, cacheKey, freshData, middleware.CacheDurations.AutobrrStatus); err == nil {
		// Also cache as stale data with longer duration
		_ = store.Set(ctx, cacheKey+":stale", freshData, autobrrStaleDataDuration)
	}

	return freshData, nil
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

	cacheKey := releasesPrefix + instanceId
	ctx := context.Background()

	// Use singleflight to prevent duplicate requests
	result, err, _ := h.sf.Do(fmt.Sprintf("releases:%s", instanceId), func() (interface{}, error) {
		return fetchDataWithCache(ctx, h.store, h.circuitBreaker, cacheKey, func() (types.ReleasesResponse, error) {
			return h.fetchReleases(instanceId)
		})
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

	releases, err := utils.SafeConvert[types.ReleasesResponse](result)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert releases response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
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

	if instanceId[:7] != "autobrr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := statsPrefix + instanceId
	ctx := context.Background()

	// Use singleflight to prevent duplicate requests
	result, err, _ := h.sf.Do(fmt.Sprintf("stats:%s", instanceId), func() (interface{}, error) {
		return fetchDataWithCache(ctx, h.store, h.circuitBreaker, cacheKey, func() (types.AutobrrStats, error) {
			return h.fetchStats(instanceId)
		})
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

	stats, err := utils.SafeConvert[types.AutobrrStats](result)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert stats response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
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

	if instanceId[:7] != "autobrr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Autobrr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Autobrr instance ID"})
		return
	}

	cacheKey := ircPrefix + instanceId
	ctx := context.Background()

	// Use singleflight to prevent duplicate requests
	result, err, _ := h.sf.Do(fmt.Sprintf("irc:%s", instanceId), func() (interface{}, error) {
		return fetchDataWithCache(ctx, h.store, h.circuitBreaker, cacheKey, func() ([]types.IRCStatus, error) {
			return h.fetchIRC(instanceId)
		})
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

	status, err := utils.SafeConvert[[]types.IRCStatus](result)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert IRC status response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
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

func (h *AutobrrHandler) fetchStats(instanceId string) (types.AutobrrStats, error) {
	autobrrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.AutobrrStats{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.AutobrrStats{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	return service.GetReleaseStats(context.Background(), autobrrConfig.URL, autobrrConfig.APIKey)
}

func (h *AutobrrHandler) fetchReleases(instanceId string) (types.ReleasesResponse, error) {
	autobrrConfig, err := h.db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return types.ReleasesResponse{}, err
	}

	if autobrrConfig == nil || autobrrConfig.URL == "" {
		return types.ReleasesResponse{}, fmt.Errorf("service not configured")
	}

	service := &autobrr.AutobrrService{
		ServiceCore: core.ServiceCore{},
	}

	return service.GetReleases(context.Background(), autobrrConfig.URL, autobrrConfig.APIKey)
}

func (h *AutobrrHandler) fetchIRC(instanceId string) ([]types.IRCStatus, error) {
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

	return service.GetIRCStatus(context.Background(), autobrrConfig.URL, autobrrConfig.APIKey)
}

// broadcastReleases broadcasts release updates to all connected SSE clients
func (h *AutobrrHandler) broadcastReleases(instanceId string, releases types.ReleasesResponse) {
	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_releases",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": releases,
		},
	}

	BroadcastHealth(health)
}

// broadcastStats broadcasts stats updates to all connected SSE clients
func (h *AutobrrHandler) broadcastStats(instanceId string, stats types.AutobrrStats) {
	health := models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "online",
		Message:     "autobrr_stats",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"autobrr": stats,
		},
	}

	BroadcastHealth(health)
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

	BroadcastHealth(health)
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
