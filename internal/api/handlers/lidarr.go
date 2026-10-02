// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/lidarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

type LidarrHandler struct {
	db     *database.DB
	poller *Poller
}

func NewLidarrHandler(db *database.DB, poller *Poller) *LidarrHandler {
	return &LidarrHandler{db: db, poller: poller}
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

	h.poller.Refresh(instanceID)

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
