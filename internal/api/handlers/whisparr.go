// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/whisparr"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	whisparrQueuePrefix       = "whisparr:queue:"
	whisparrStaleDataDuration = 5 * time.Minute
)

type WhisparrHandler struct {
	db              *database.DB
	cache           cache.Store
	bc              *Broadcaster
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewWhisparrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *WhisparrHandler {
	return &WhisparrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastQueueHash:  make(map[string]string),
	}
}

func (h *WhisparrHandler) GetQueue(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "whisparr", "Whisparr")
	if !ok {
		return
	}

	cacheKey := whisparrQueuePrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.WhisparrQueueResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.WhisparrStatus,
		StaleTTL:       whisparrStaleDataDuration,
		Fetch: func() (types.WhisparrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
	})
	if err != nil {
		if handleArrFetchError(c, err, "Whisparr", instanceID, "queue") {
			return
		}
		return
	}

	compareAndLogArrQueueChanges(
		h.lastQueueHash,
		&h.lastQueueHashMu,
		"Whisparr",
		instanceID,
		result.TotalRecords,
		wrapWhisparrQueue(&result),
	)
	if result.Records != nil {
		publishInternalServiceUpdate(h.bc, buildWhisparrQueueServiceUpdate(instanceID, &result))
	}

	c.JSON(http.StatusOK, result)
}

func (h *WhisparrHandler) fetchQueue(ctx context.Context, instanceID string) (types.WhisparrQueueResponse, error) {
	whisparrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "whisparr")
	if err != nil {
		return types.WhisparrQueueResponse{}, err
	}

	service := &whisparr.WhisparrService{}
	records, err := service.GetQueueForHealth(ctx, whisparrConfig.URL, whisparrConfig.APIKey)
	if err != nil {
		return types.WhisparrQueueResponse{}, err
	}

	return types.WhisparrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}, nil
}

func (h *WhisparrHandler) DeleteQueueItem(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "whisparr", "Whisparr")
	if !ok {
		return
	}

	queueID := c.Param("id")
	if queueID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.WhisparrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()
	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceID, queueID, options)
	})
	if handleQueueDeleteError(c, err, "Whisparr", instanceID, queueID) {
		return
	}

	cacheKey := whisparrQueuePrefix + instanceID
	refreshQueueAfterDelete(
		ctx,
		h.cache,
		h.circuitBreaker,
		cacheKey,
		middleware.CacheDurations.WhisparrStatus,
		whisparrStaleDataDuration,
		func() (types.WhisparrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
		func(result *types.WhisparrQueueResponse) {
			publishInternalServiceUpdate(h.bc, buildWhisparrQueueServiceUpdate(instanceID, result))
		},
		"Whisparr",
		instanceID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *WhisparrHandler) deleteQueueItem(
	ctx context.Context,
	instanceID, queueID string,
	options types.WhisparrQueueDeleteOptions,
) error {
	whisparrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "whisparr")
	if err != nil {
		return err
	}

	service := &whisparr.WhisparrService{}
	return service.DeleteQueueItem(ctx, whisparrConfig.URL, whisparrConfig.APIKey, queueID, options)
}
