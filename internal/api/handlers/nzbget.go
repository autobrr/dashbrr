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
	"github.com/autobrr/dashbrr/internal/services/nzbget"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	nzbgetSummaryPrefix     = "nzbget:summary:"
	nzbgetStaleDataDuration = 5 * time.Minute
)

type NzbgetHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewNzbgetHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *NzbgetHandler {
	return &NzbgetHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *NzbgetHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "nzbget", "NZBGet")
	if !ok {
		return
	}

	cacheKey := nzbgetSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.NzbgetSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.NzbgetStatus,
		StaleTTL:       nzbgetStaleDataDuration,
		Fetch: func() (types.NzbgetSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceID).
			Msg("[NZBGet] Failed to fetch summary")
		c.JSON(statusFromNzbgetError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *NzbgetHandler) fetchSummary(ctx context.Context, instanceID string) (types.NzbgetSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "nzbget")
	if err != nil {
		return types.NzbgetSummaryResponse{}, err
	}

	service := nzbget.NewNzbgetService().(*nzbget.NzbgetService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *NzbgetHandler) compareAndLogSummaryChanges(instanceID string, summary *types.NzbgetSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createNzbgetSummaryHash(summary)
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
		Bool("downloadPaused", summary.Status.DownloadPaused).
		Int("queue", len(summary.Queue)).
		Int("failed", summary.FailedCount).
		Str("change", change).
		Msg("[NZBGet] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *NzbgetHandler) broadcastSummary(instanceID string, summary *types.NzbgetSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildNzbgetSummaryServiceUpdate(instanceID, summary))
}

func createNzbgetSummaryHash(summary *types.NzbgetSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"%t:%t:%t:%t:%d:%d|",
		summary.Status.DownloadPaused,
		summary.Status.PostPaused,
		summary.Status.ScanPaused,
		summary.Status.QuotaReached,
		len(summary.Queue),
		summary.FailedCount,
	)

	for _, item := range summary.Queue {
		fmt.Fprintf(&builder, "q:%d:%s:%s|", item.NZBID, item.Status, item.NZBName)
	}
	for _, failure := range summary.RecentFailures {
		fmt.Fprintf(&builder, "f:%d:%s|", failure.NZBID, failure.Status)
	}

	return builder.String()
}

func statusFromNzbgetError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var nzbErr *nzbget.ErrNzbget
	if errors.As(err, &nzbErr) && nzbErr.HttpCode > 0 {
		return normalizeUpstreamStatus(nzbErr.HttpCode)
	}

	return http.StatusInternalServerError
}
