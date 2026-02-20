// Copyright (c) 2026, s0up and the autobrr contributors.
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
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/jellyfin"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	jellyfinSummaryPrefix     = "jellyfin:summary:"
	jellyfinStaleDataDuration = 5 * time.Minute
)

type JellyfinHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewJellyfinHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *JellyfinHandler {
	return &JellyfinHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *JellyfinHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "jellyfin", "Jellyfin")
	if !ok {
		return
	}

	cacheKey := jellyfinSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.JellyfinSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.JellyfinStatus,
		StaleTTL:       jellyfinStaleDataDuration,
		Fetch: func() (types.JellyfinSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[Jellyfin] Failed to fetch summary")
		c.JSON(statusFromJellyfinError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *JellyfinHandler) fetchSummary(ctx context.Context, instanceID string) (types.JellyfinSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "jellyfin")
	if err != nil {
		return types.JellyfinSummaryResponse{}, err
	}

	service := jellyfin.NewJellyfinService().(*jellyfin.JellyfinService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *JellyfinHandler) compareAndLogSummaryChanges(instanceID string, summary *types.JellyfinSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createJellyfinSummaryHash(summary)
	lastHash := h.lastSummaryHash[instanceID]
	if currentHash == lastHash {
		return
	}

	change := "summary_updated"
	if lastHash == "" {
		change = "initial_summary"
	}

	log.Debug().
		Str("instanceId", instanceID).
		Str("version", summary.System.Version).
		Int("sessions", len(summary.Sessions)).
		Int("transcoding", countJellyfinTranscoding(summary.Sessions)).
		Str("change", change).
		Msg("[Jellyfin] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *JellyfinHandler) broadcastSummary(instanceID string, summary *types.JellyfinSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildJellyfinSummaryServiceUpdate(instanceID, summary))
}

func createJellyfinSummaryHash(summary *types.JellyfinSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "%s:%s:%d|", summary.System.ServerName, summary.System.Version, len(summary.Sessions))
	for _, session := range summary.Sessions {
		itemName := ""
		position := int64(0)
		playMethod := ""
		paused := false
		if session.NowPlayingItem != nil {
			itemName = session.NowPlayingItem.Name
		}
		if session.PlayState != nil {
			position = session.PlayState.PositionTicks
			playMethod = session.PlayState.PlayMethod
			paused = session.PlayState.IsPaused
		}
		fmt.Fprintf(&builder, "s:%s:%s:%s:%d:%t:%s:%t|", session.ID, session.UserName, itemName, position, paused, playMethod, session.TranscodingInfo != nil)
	}

	return builder.String()
}

func statusFromJellyfinError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var jellyErr *jellyfin.ErrJellyfin
	if errors.As(err, &jellyErr) && jellyErr.HttpCode > 0 {
		return normalizeUpstreamStatus(jellyErr.HttpCode)
	}

	return http.StatusInternalServerError
}
