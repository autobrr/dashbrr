// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/whisparr"
	"github.com/autobrr/dashbrr/internal/types"
)

type WhisparrHandler struct {
	db     *database.DB
	poller *Poller
}

func NewWhisparrHandler(db *database.DB, poller *Poller) *WhisparrHandler {
	return &WhisparrHandler{db: db, poller: poller}
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

	h.poller.Refresh(instanceID)

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
