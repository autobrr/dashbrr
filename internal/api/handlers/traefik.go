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
	"github.com/autobrr/dashbrr/internal/services/traefik"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	traefikSummaryPrefix     = "traefik:summary:"
	traefikStaleDataDuration = 2 * time.Minute
)

type TraefikHandler struct {
	db             *database.DB
	cache          cache.Store
	bc             *Broadcaster
	circuitBreaker *resilience.CircuitBreaker

	lastSummaryHash   map[string]string
	lastSummaryHashMu sync.Mutex
}

func NewTraefikHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *TraefikHandler {
	return &TraefikHandler{
		db:              db,
		cache:           cache,
		bc:              bc,
		circuitBreaker:  resilience.NewCircuitBreaker(5, time.Minute),
		lastSummaryHash: make(map[string]string),
	}
}

func (h *TraefikHandler) GetSummary(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "traefik", "Traefik")
	if !ok {
		return
	}

	cacheKey := traefikSummaryPrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.TraefikSummaryResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.TraefikStatus,
		StaleTTL:       traefikStaleDataDuration,
		Fetch: func() (types.TraefikSummaryResponse, error) {
			return h.fetchSummary(ctx, instanceID)
		},
	})
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[Traefik] Failed to fetch summary")
		c.JSON(statusFromTraefikError(err), gin.H{"error": err.Error()})
		return
	}

	h.compareAndLogSummaryChanges(instanceID, &result)
	h.broadcastSummary(instanceID, &result)

	c.JSON(http.StatusOK, result)
}

func (h *TraefikHandler) fetchSummary(ctx context.Context, instanceID string) (types.TraefikSummaryResponse, error) {
	serviceConfig, err := requireServiceConfig(ctx, h.db, instanceID, "traefik")
	if err != nil {
		return types.TraefikSummaryResponse{}, err
	}

	service := traefik.NewTraefikService().(*traefik.TraefikService)
	return service.GetSummary(ctx, serviceConfig.URL, serviceConfig.APIKey)
}

func (h *TraefikHandler) compareAndLogSummaryChanges(instanceID string, summary *types.TraefikSummaryResponse) {
	h.lastSummaryHashMu.Lock()
	defer h.lastSummaryHashMu.Unlock()

	currentHash := createTraefikSummaryHash(summary)
	lastHash := h.lastSummaryHash[instanceID]
	if currentHash == lastHash {
		return
	}

	change := "summary_updated"
	if lastHash == "" {
		change = "initial_summary"
	}

	overview := summary.Overview
	log.Debug().
		Str("instanceId", instanceID).
		Int("httpRouters", sectionTotal(overview.HTTP.Routers)).
		Int("httpRouterWarnings", sectionWarnings(overview.HTTP.Routers)).
		Int("httpRouterErrors", sectionErrors(overview.HTTP.Routers)).
		Int("providers", len(overview.Providers)).
		Int("issueRouters", len(summary.IssueRouters)).
		Str("change", change).
		Msg("[Traefik] Summary changed")

	h.lastSummaryHash[instanceID] = currentHash
}

func (h *TraefikHandler) broadcastSummary(instanceID string, summary *types.TraefikSummaryResponse) {
	publishInternalServiceUpdate(h.bc, buildTraefikSummaryServiceUpdate(instanceID, summary))
}

func createTraefikSummaryHash(summary *types.TraefikSummaryResponse) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"http:%d:%d:%d|svc:%d:%d:%d|mid:%d:%d:%d|providers:%d|issues:%d|",
		sectionTotal(summary.Overview.HTTP.Routers),
		sectionWarnings(summary.Overview.HTTP.Routers),
		sectionErrors(summary.Overview.HTTP.Routers),
		sectionTotal(summary.Overview.HTTP.Services),
		sectionWarnings(summary.Overview.HTTP.Services),
		sectionErrors(summary.Overview.HTTP.Services),
		sectionTotal(summary.Overview.HTTP.Middlewares),
		sectionWarnings(summary.Overview.HTTP.Middlewares),
		sectionErrors(summary.Overview.HTTP.Middlewares),
		len(summary.Overview.Providers),
		len(summary.IssueRouters),
	)
	for _, router := range summary.IssueRouters {
		fmt.Fprintf(&builder, "r:%s:%s:%s|", router.Name, router.Provider, router.Status)
	}

	return builder.String()
}

func statusFromTraefikError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}

	var traefikErr *traefik.ErrTraefik
	if errors.As(err, &traefikErr) && traefikErr.HttpCode > 0 {
		return normalizeUpstreamStatus(traefikErr.HttpCode)
	}

	return http.StatusInternalServerError
}
