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
	"github.com/autobrr/dashbrr/internal/services/readarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	readarrQueuePrefix       = "readarr:queue:"
	readarrStaleDataDuration = 5 * time.Minute
)

type ReadarrHandler struct {
	db              *database.DB
	cache           cache.Store
	bc              *Broadcaster
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewReadarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *ReadarrHandler {
	return &ReadarrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastQueueHash:  make(map[string]string),
	}
}

func (h *ReadarrHandler) GetQueue(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "readarr", "Readarr")
	if !ok {
		return
	}

	cacheKey := readarrQueuePrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.ReadarrQueueResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.ReadarrStatus,
		StaleTTL:       readarrStaleDataDuration,
		Fetch: func() (types.ReadarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
	})
	if err != nil {
		if handleArrFetchError(c, err, "Readarr", instanceID, "queue") {
			return
		}
		return
	}

	compareAndLogArrQueueChanges(
		h.lastQueueHash,
		&h.lastQueueHashMu,
		"Readarr",
		instanceID,
		result.TotalRecords,
		wrapReadarrQueue(&result),
	)
	if result.Records != nil {
		publishInternalServiceUpdate(h.bc, buildReadarrQueueServiceUpdate(instanceID, &result))
	}

	c.JSON(http.StatusOK, result)
}

func (h *ReadarrHandler) fetchQueue(ctx context.Context, instanceID string) (types.ReadarrQueueResponse, error) {
	readarrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "readarr")
	if err != nil {
		return types.ReadarrQueueResponse{}, err
	}

	service := &readarr.ReadarrService{}
	records, err := service.GetQueueForHealth(ctx, readarrConfig.URL, readarrConfig.APIKey)
	if err != nil {
		return types.ReadarrQueueResponse{}, err
	}

	return types.ReadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}, nil
}

func (h *ReadarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "readarr", "Readarr")
	if !ok {
		return
	}

	queueID := c.Param("id")
	if queueID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.ReadarrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()
	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceID, queueID, options)
	})
	if handleQueueDeleteError(c, err, "Readarr", instanceID, queueID) {
		return
	}

	cacheKey := readarrQueuePrefix + instanceID
	refreshQueueAfterDelete(
		ctx,
		h.cache,
		h.circuitBreaker,
		cacheKey,
		middleware.CacheDurations.ReadarrStatus,
		readarrStaleDataDuration,
		func() (types.ReadarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
		func(result *types.ReadarrQueueResponse) {
			publishInternalServiceUpdate(h.bc, buildReadarrQueueServiceUpdate(instanceID, result))
		},
		"Readarr",
		instanceID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *ReadarrHandler) deleteQueueItem(
	ctx context.Context,
	instanceID, queueID string,
	options types.ReadarrQueueDeleteOptions,
) error {
	readarrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "readarr")
	if err != nil {
		return err
	}

	service := &readarr.ReadarrService{}
	return service.DeleteQueueItem(ctx, readarrConfig.URL, readarrConfig.APIKey, queueID, options)
}
