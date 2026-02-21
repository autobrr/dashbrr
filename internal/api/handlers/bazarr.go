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
	"github.com/autobrr/dashbrr/internal/services/bazarr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	bazarrSummaryPrefix     = "bazarr:summary:"
	bazarrStaleDataDuration = 5 * time.Minute
)

type BazarrHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewBazarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *BazarrHandler {
	return &BazarrHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *BazarrHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "bazarr", "Bazarr")
	if !ok {
		return
	}

	cacheKey := bazarrSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.BazarrSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.BazarrStatus,
		StaleTTL:       bazarrStaleDataDuration,
		Fetch: func() (types.BazarrSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceID).
			Msg("[Bazarr] Failed to fetch summary")
		c.JSON(statusFromBazarrError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *BazarrHandler) fetchSummary(ctx context.Context, instanceID string) (types.BazarrSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "bazarr")
	if err != nil {
		return types.BazarrSummaryResponse{}, err
	}

	service := bazarr.NewBazarrService().(*bazarr.BazarrService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *BazarrHandler) compareAndLogSummaryChanges(instanceID string, summary *types.BazarrSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createBazarrSummaryHash(summary)
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
		Int("episodes", summary.Badges.Episodes).
		Int("movies", summary.Badges.Movies).
		Int("providers", len(summary.Providers)).
		Int("healthIssues", len(summary.HealthIssues)).
		Str("change", change).
		Msg("[Bazarr] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *BazarrHandler) broadcastSummary(instanceID string, summary *types.BazarrSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildBazarrSummaryServiceUpdate(instanceID, summary))
}

func createBazarrSummaryHash(summary *types.BazarrSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"%d:%d:%d:%d:%s:%s:%d|",
		summary.Badges.Episodes,
		summary.Badges.Movies,
		summary.Badges.Providers,
		summary.Badges.Status,
		summary.Badges.SonarrSignalR,
		summary.Badges.RadarrSignalR,
		summary.Badges.Announcements,
	)

	for _, provider := range summary.Providers {
		fmt.Fprintf(&builder, "p:%s:%s:%s|", provider.Name, provider.Status, provider.Retry)
	}
	for _, issue := range summary.HealthIssues {
		fmt.Fprintf(&builder, "h:%s:%s|", issue.Object, issue.Issue)
	}

	return builder.String()
}

func statusFromBazarrError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var bazarrErr *bazarr.ErrBazarr
	if errors.As(err, &bazarrErr) && bazarrErr.HttpCode > 0 {
		return normalizeUpstreamStatus(bazarrErr.HttpCode)
	}

	return http.StatusInternalServerError
}
