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
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/sabnzbd"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	sabnzbdSummaryPrefix     = "sabnzbd:summary:"
	sabnzbdStaleDataDuration = 5 * time.Minute
)

type SabnzbdHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewSabnzbdHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *SabnzbdHandler {
	return &SabnzbdHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *SabnzbdHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "sabnzbd", "SABnzbd")
	if !ok {
		return
	}

	cacheKey := sabnzbdSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.SabnzbdSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.SabnzbdStatus,
		StaleTTL:       sabnzbdStaleDataDuration,
		Fetch: func() (types.SabnzbdSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceID).
			Msg("[SABnzbd] Failed to fetch summary")
		c.JSON(statusFromSabnzbdError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *SabnzbdHandler) fetchSummary(ctx context.Context, instanceID string) (types.SabnzbdSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "sabnzbd")
	if err != nil {
		return types.SabnzbdSummaryResponse{}, err
	}

	service := sabnzbd.NewSabnzbdService().(*sabnzbd.SabnzbdService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *SabnzbdHandler) compareAndLogSummaryChanges(instanceID string, summary *types.SabnzbdSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createSabnzbdSummaryHash(summary)
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
		Str("status", summary.Queue.Status).
		Str("queue", string(summary.Queue.NoOfSlots)).
		Int("failed", summary.FailedCount).
		Str("warnings", summary.Queue.HaveWarnings).
		Str("change", change).
		Msg("[SABnzbd] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *SabnzbdHandler) broadcastSummary(instanceID string, summary *types.SabnzbdSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildSabnzbdSummaryServiceUpdate(instanceID, summary))
}

func createSabnzbdSummaryHash(summary *types.SabnzbdSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"%s:%s:%s:%s:%d|",
		summary.Queue.Status,
		summary.Queue.Speed,
		string(summary.Queue.NoOfSlots),
		summary.Queue.HaveWarnings,
		summary.FailedCount,
	)

	for _, slot := range summary.Queue.Slots {
		fmt.Fprintf(&builder, "q:%s:%s:%s|", slot.NzoID, slot.Status, slot.Percentage)
	}
	for _, slot := range summary.RecentFailures {
		fmt.Fprintf(&builder, "f:%s:%s|", slot.NzoID, slot.Status)
	}

	return builder.String()
}

func statusFromSabnzbdError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var sabErr *sabnzbd.ErrSabnzbd
	if errors.As(err, &sabErr) && sabErr.HttpCode > 0 {
		return normalizeUpstreamStatus(sabErr.HttpCode)
	}

	return http.StatusInternalServerError
}
