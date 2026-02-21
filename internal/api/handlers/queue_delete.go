// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
)

type queueDeleteQueryOptions struct {
	RemoveFromClient bool
	Blocklist        bool
	SkipRedownload   bool
	ChangeCategory   bool
}

func queueDeleteOptionsFromQuery(c *gin.Context) queueDeleteQueryOptions {
	return queueDeleteQueryOptions{
		RemoveFromClient: c.Query("removeFromClient") == "true",
		Blocklist:        c.Query("blocklist") == "true",
		SkipRedownload:   c.Query("skipRedownload") == "true",
		ChangeCategory:   c.Query("changeCategory") == "true",
	}
}

func handleQueueDeleteError(c *gin.Context, err error, serviceName, instanceID, queueID string) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrServiceNotConfigured) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return true
	}

	if arrErr, ok := err.(*arr.ErrArr); ok {
		log.Error().
			Err(arrErr).
			Str("instanceId", instanceID).
			Str("queueId", queueID).
			Msg(fmt.Sprintf("[%s] Failed to delete queue item", serviceName))

		if arrErr.HttpCode > 0 {
			c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
			return true
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete queue item: %v", err)})
	return true
}

func refreshQueueAfterDelete[T any](
	ctx context.Context,
	store cache.Store,
	circuitBreaker *resilience.CircuitBreaker,
	cacheKey string,
	freshTTL time.Duration,
	staleTTL time.Duration,
	fetch func() (T, error),
	broadcast func(*T),
	serviceName string,
	instanceID string,
) {
	if err := DeleteSWRCacheKeys(ctx, store, cacheKey); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceID).
			Msgf("[%s] Failed to clear queue cache", serviceName)
	}

	result, err := FetchWithSWRCache(ctx, SWRCacheOptions[T]{
		Store:          store,
		CircuitBreaker: circuitBreaker,
		Key:            cacheKey,
		FreshTTL:       freshTTL,
		StaleTTL:       staleTTL,
		Fetch:          fetch,
	})
	if err != nil {
		return
	}

	broadcast(&result)
}
