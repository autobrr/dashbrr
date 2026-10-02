// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
)

type ReadarrHandler struct {
	db     *database.DB
	poller *Poller
}

func NewReadarrHandler(db *database.DB, poller *Poller) *ReadarrHandler {
	return &ReadarrHandler{db: db, poller: poller}
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

	options := queueDeleteOptionsFromQuery(c)

	ctx := c.Request.Context()
	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceID, queueID, options)
	})
	if handleQueueDeleteError(c, err, "Readarr", instanceID, queueID) {
		return
	}

	h.poller.Refresh(instanceID)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *ReadarrHandler) deleteQueueItem(
	ctx context.Context,
	instanceID, queueID string,
	options arr.QueueDeleteOptions,
) error {
	readarrConfig, err := requireServiceConfig(ctx, h.db, instanceID, "readarr")
	if err != nil {
		return err
	}

	return arr.Readarr.DeleteQueueItem(ctx, readarrConfig.URL, readarrConfig.APIKey, queueID, options)
}
