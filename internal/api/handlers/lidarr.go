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
	"github.com/autobrr/dashbrr/internal/services/lidarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	lidarrQueuePrefix       = "lidarr:queue:"
	lidarrStaleDataDuration = 5 * time.Minute
)

type LidarrHandler struct {
	db              *database.DB
	cache           cache.Store
	bc              *Broadcaster
	circuitBreaker  *resilience.CircuitBreaker
	lastQueueHash   map[string]string
	lastQueueHashMu sync.Mutex
}

func NewLidarrHandler(db *database.DB, cache cache.Store, bc *Broadcaster) *LidarrHandler {
	return &LidarrHandler{
		db:             db,
		cache:          cache,
		bc:             bc,
		circuitBreaker: resilience.NewCircuitBreaker(5, 1*time.Minute),
		lastQueueHash:  make(map[string]string),
	}
}

func (h *LidarrHandler) GetQueue(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "lidarr", "Lidarr")
	if !ok {
		return
	}

	cacheKey := lidarrQueuePrefix + instanceID
	ctx := c.Request.Context()

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[types.LidarrQueueResponse]{
		Store:          h.cache,
		CircuitBreaker: h.circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       middleware.CacheDurations.LidarrStatus,
		StaleTTL:       lidarrStaleDataDuration,
		Fetch: func() (types.LidarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
	})
	if err != nil {
		if handleArrFetchError(c, err, "Lidarr", instanceID, "queue") {
			return
		}
		return
	}

	compareAndLogArrQueueChanges(
		h.lastQueueHash,
		&h.lastQueueHashMu,
		"Lidarr",
		instanceID,
		result.TotalRecords,
		wrapLidarrQueue(&result),
	)
	if result.Records != nil {
		publishInternalServiceUpdate(h.bc, buildLidarrQueueServiceUpdate(instanceID, &result))
	}

	c.JSON(http.StatusOK, result)
}

func (h *LidarrHandler) fetchQueue(ctx context.Context, instanceID string) (types.LidarrQueueResponse, error) {
	lidarrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "lidarr")
	if err != nil {
		return types.LidarrQueueResponse{}, err
	}

	service := &lidarr.LidarrService{}
	records, err := service.GetQueueForHealth(ctx, lidarrConfig.URL, lidarrConfig.APIKey)
	if err != nil {
		return types.LidarrQueueResponse{}, err
	}

	return types.LidarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}, nil
}

func (h *LidarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "lidarr", "Lidarr")
	if !ok {
		return
	}

	queueID := c.Param("id")
	if queueID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.LidarrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()
	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceID, queueID, options)
	})
	if handleQueueDeleteError(c, err, "Lidarr", instanceID, queueID) {
		return
	}

	cacheKey := lidarrQueuePrefix + instanceID
	refreshQueueAfterDelete(
		ctx,
		h.cache,
		h.circuitBreaker,
		cacheKey,
		middleware.CacheDurations.LidarrStatus,
		lidarrStaleDataDuration,
		func() (types.LidarrQueueResponse, error) {
			return h.fetchQueue(ctx, instanceID)
		},
		func(result *types.LidarrQueueResponse) {
			publishInternalServiceUpdate(h.bc, buildLidarrQueueServiceUpdate(instanceID, result))
		},
		"Lidarr",
		instanceID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *LidarrHandler) deleteQueueItem(
	ctx context.Context,
	instanceID, queueID string,
	options types.LidarrQueueDeleteOptions,
) error {
	lidarrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "lidarr")
	if err != nil {
		return err
	}

	service := &lidarr.LidarrService{}
	return service.DeleteQueueItem(ctx, lidarrConfig.URL, lidarrConfig.APIKey, queueID, options)
}
