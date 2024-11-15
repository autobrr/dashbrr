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
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	plexCachePrefix       = "plex:sessions:"
	plexStaleDataDuration = 5 * time.Minute
)

type PlexHandler struct {
	db                *database.DB
	cache             cache.Store
	sf                singleflight.Group
	circuitBreaker    *resilience.CircuitBreaker
	lastSessionHash   map[string]string
	lastSessionHashMu sync.Mutex
}

func NewPlexHandler(db *database.DB, cache cache.Store) *PlexHandler {
	return &PlexHandler{
		db:              db,
		cache:           cache,
		circuitBreaker:  resilience.NewCircuitBreaker(5, 1*time.Minute), // 5 failures within 1 minute will open the circuit
		lastSessionHash: make(map[string]string),
	}
}

// fetchDataWithCache implements a stale-while-revalidate pattern
func (h *PlexHandler) fetchDataWithCache(ctx context.Context, cacheKey string, fetchFn func() (interface{}, error)) (interface{}, error) {
	var data interface{}

	// Try to get from cache first
	err := h.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		// Data found in cache
		go func() {
			// Refresh cache in background if close to expiration
			if time.Now().After(time.Now().Add(-middleware.CacheDurations.PlexSessions + 5*time.Second)) {
				if newData, err := fetchFn(); err == nil {
					h.cache.Set(ctx, cacheKey, newData, middleware.CacheDurations.PlexSessions)
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
	if err := h.cache.Set(ctx, cacheKey, data, middleware.CacheDurations.PlexSessions); err == nil {
		// Also cache as stale data with longer duration
		h.cache.Set(ctx, cacheKey+":stale", data, plexStaleDataDuration)
	}

	return data, nil
}

func (h *PlexHandler) GetPlexSessions(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Plex instance
	if instanceId[:4] != "plex" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Plex instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Plex instance ID"})
		return
	}

	cacheKey := plexCachePrefix + instanceId
	ctx := context.Background()

	// Use singleflight to prevent duplicate requests
	sfKey := fmt.Sprintf("sessions:%s", instanceId)
	sessionsI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchDataWithCache(ctx, cacheKey, func() (interface{}, error) {
			return h.fetchSessions(ctx, instanceId)
		})
	})

	if err != nil {
		if err.Error() == "service not configured" {
			// Return empty response for unconfigured service
			emptyResponse := &types.PlexSessionsResponse{}
			emptyResponse.MediaContainer.Size = 0
			emptyResponse.MediaContainer.Metadata = []types.PlexSession{}
			c.JSON(http.StatusOK, emptyResponse)
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Request timeout while fetching Plex sessions")
		} else {
			log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Plex sessions")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	sessions := sessionsI.(*types.PlexSessionsResponse)

	if sessions != nil {
		h.compareAndLogSessionChanges(instanceId, sessions)
		h.broadcastPlexSessions(instanceId, sessions)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Retrieved empty Plex sessions")
	}

	c.JSON(http.StatusOK, sessions)
}

func (h *PlexHandler) fetchSessions(ctx context.Context, instanceId string) (*types.PlexSessionsResponse, error) {
	plexConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
	if err != nil {
		return nil, err
	}

	if plexConfig == nil || plexConfig.URL == "" {
		return nil, fmt.Errorf("service not configured")
	}

	service := &plex.PlexService{}
	sessions, err := service.GetSessions(ctx, plexConfig.URL, plexConfig.APIKey)
	if err != nil {
		return nil, err
	}

	if sessions == nil {
		return nil, nil
	}

	// Initialize empty metadata if nil
	if sessions.MediaContainer.Metadata == nil {
		sessions.MediaContainer.Metadata = []types.PlexSession{}
	}

	return sessions, nil
}

// broadcastPlexSessions broadcasts Plex session updates to all connected SSE clients
func (h *PlexHandler) broadcastPlexSessions(instanceId string, sessions *types.PlexSessionsResponse) {
	// Use the existing BroadcastHealth function with a special message type
	BroadcastHealth(models.ServiceHealth{
		ServiceID:   instanceId,
		Status:      "ok",
		Message:     "plex_sessions",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"plex": map[string]interface{}{
				"sessions": sessions.MediaContainer.Metadata,
			},
		},
		Details: map[string]interface{}{
			"plex": map[string]interface{}{
				"activeStreams": len(sessions.MediaContainer.Metadata),
				"transcoding":   len(filterTranscodingSessions(sessions.MediaContainer.Metadata)),
			},
		},
	})
}

// filterTranscodingSessions returns sessions that are being transcoded
func filterTranscodingSessions(sessions []types.PlexSession) []types.PlexSession {
	transcoding := make([]types.PlexSession, 0)
	for _, session := range sessions {
		if session.TranscodeSession != nil {
			transcoding = append(transcoding, session)
		}
	}
	return transcoding
}

// createSessionHash generates a unique hash representing the current state of Plex sessions
// The hash includes key session details like session key, media title, user, and playback state
// This allows for efficient detection of session changes without deep comparison
// Also helps reduce log spam by only logging when meaningful changes occur in sessions
func createSessionHash(sessions *types.PlexSessionsResponse) string {
	if sessions == nil || len(sessions.MediaContainer.Metadata) == 0 {
		return ""
	}

	// Create a string that represents the current state
	var sb strings.Builder
	for _, session := range sessions.MediaContainer.Metadata {
		// Include session identity and player state
		fmt.Fprintf(&sb, "%s:%s:%s:%s:%s,",
			session.SessionKey,
			session.GrandparentTitle,
			session.Title,
			session.User.Title,
			session.Player.State)
	}
	return sb.String()
}

func (h *PlexHandler) detectSessionChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_sessions"
	}

	oldSessions := strings.Split(oldHash, ",")
	newSessions := strings.Split(newHash, ",")

	if len(oldSessions) < len(newSessions) {
		return "stream_started"
	} else if len(oldSessions) > len(newSessions) {
		return "stream_ended"
	}

	return "state_changed"
}

// compareAndLogSessionChanges tracks and logs changes in Plex media sessions
// It compares the current session state with the previous state for a specific Plex instance
// Helps detect session state changes like new streams starting, streams ending, or playback state changes
func (h *PlexHandler) compareAndLogSessionChanges(instanceId string, sessions *types.PlexSessionsResponse) {
	h.lastSessionHashMu.Lock()
	defer h.lastSessionHashMu.Unlock()

	currentHash := createSessionHash(sessions)
	lastHash := h.lastSessionHash[instanceId]

	if currentHash != lastHash {
		// Detect specific changes
		changes := h.detectSessionChanges(lastHash, currentHash)

		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Str("change", changes).
			Msg("[Plex] Sessions changed")

		h.lastSessionHash[instanceId] = currentHash
	}
}
