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
	"github.com/autobrr/dashbrr/internal/services/uptimekuma"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	uptimeKumaSummaryPrefix     = "uptimekuma:summary:"
	uptimeKumaStaleDataDuration = 2 * time.Minute
)

type UptimeKumaHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewUptimeKumaHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *UptimeKumaHandler {
	return &UptimeKumaHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *UptimeKumaHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "uptimekuma", "Uptime Kuma")
	if !ok {
		return
	}

	cacheKey := uptimeKumaSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.UptimeKumaSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.UptimeKumaStatus,
		StaleTTL:       uptimeKumaStaleDataDuration,
		Fetch: func() (types.UptimeKumaSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[UptimeKuma] Failed to fetch summary")
		c.JSON(statusFromUptimeKumaError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *UptimeKumaHandler) fetchSummary(ctx context.Context, instanceID string) (types.UptimeKumaSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "uptimekuma")
	if err != nil {
		return types.UptimeKumaSummaryResponse{}, err
	}

	service := uptimekuma.NewUptimeKumaService().(*uptimekuma.UptimeKumaService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *UptimeKumaHandler) compareAndLogSummaryChanges(instanceID string, summary *types.UptimeKumaSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createUptimeKumaSummaryHash(summary)
	lastHash := h.lastSummaryHash[instanceID]
	if currentHash == lastHash {
		return
	}

	change := "summary_updated"
	if lastHash == "" {
		change = "initial_summary"
	}

	total, up, down, pending, maintenance := countUptimeKumaStates(summary.Monitors)

	log.Debug().
		Str("instanceId", instanceID).
		Int("total", total).
		Int("up", up).
		Int("down", down).
		Int("pending", pending).
		Int("maintenance", maintenance).
		Str("change", change).
		Msg("[UptimeKuma] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *UptimeKumaHandler) broadcastSummary(instanceID string, summary *types.UptimeKumaSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildUptimeKumaSummaryServiceUpdate(instanceID, summary))
}

func createUptimeKumaSummaryHash(summary *types.UptimeKumaSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "%d|", len(summary.Monitors))
	for _, monitor := range summary.Monitors {
		fmt.Fprintf(&builder, "m:%s:%s:%s:%d|", monitor.ID, monitor.Name, monitor.Status, monitor.ResponseTimeMs)
	}
	return builder.String()
}

func statusFromUptimeKumaError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var kumaErr *uptimekuma.ErrUptimeKuma
	if errors.As(err, &kumaErr) && kumaErr.HttpCode > 0 {
		return normalizeUpstreamStatus(kumaErr.HttpCode)
	}

	return http.StatusInternalServerError
}
