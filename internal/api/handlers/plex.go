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
	"github.com/autobrr/dashbrr/internal/types"
)

const plexCachePrefix = "plex:sessions:"

type PlexHandler struct {
	db                *database.DB
	cache             cache.Store
	sf                singleflight.Group
	lastSessionHash   map[string]string
	lastSessionHashMu sync.Mutex
}

func NewPlexHandler(db *database.DB, cache cache.Store) *PlexHandler {
	return &PlexHandler{
		db:              db,
		cache:           cache,
		lastSessionHash: make(map[string]string),
	}
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

	// Try to get from cache first
	var sessions *types.PlexSessionsResponse
	err := h.cache.Get(ctx, cacheKey, &sessions)
	if err == nil && sessions != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Msg("Serving Plex sessions from cache")
		c.JSON(http.StatusOK, sessions)

		// Refresh cache in background using singleflight
		go func() {
			refreshKey := fmt.Sprintf("sessions_refresh:%s", instanceId)
			_, _, _ = h.sf.Do(refreshKey, func() (interface{}, error) {
				h.refreshSessionsCache(instanceId, cacheKey)
				return nil, nil
			})
		}()
		return
	}

	// If not in cache or invalid cache data, fetch from service using singleflight
	sfKey := fmt.Sprintf("sessions:%s", instanceId)
	sessionsI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		return h.fetchAndCacheSessions(ctx, instanceId, cacheKey)
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

	sessions = sessionsI.(*types.PlexSessionsResponse)

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

func (h *PlexHandler) fetchAndCacheSessions(ctx context.Context, instanceId, cacheKey string) (*types.PlexSessionsResponse, error) {
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

	// Cache the results using the centralized cache duration
	if err := h.cache.Set(ctx, cacheKey, sessions, middleware.CacheDurations.PlexSessions); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Plex sessions")
	}

	return sessions, nil
}

func (h *PlexHandler) refreshSessionsCache(instanceId, cacheKey string) {
	ctx := context.Background()
	sessions, err := h.fetchAndCacheSessions(ctx, instanceId, cacheKey)
	if err != nil && err.Error() != "service not configured" {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Plex sessions cache")
		return
	}

	if sessions != nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("size", sessions.MediaContainer.Size).
			Msg("Successfully refreshed Plex sessions cache")

		// Broadcast sessions update via SSE
		h.broadcastPlexSessions(instanceId, sessions)
	} else {
		log.Debug().
			Str("instanceId", instanceId).
			Msg("Refreshed cache with empty Plex sessions")
	}
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
