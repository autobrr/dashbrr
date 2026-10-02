// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/types"
)

type RadarrHandler struct {
	db     *database.DB
	poller *Poller
}

func NewRadarrHandler(db *database.DB, poller *Poller) *RadarrHandler {
	return &RadarrHandler{db: db, poller: poller}
}

// DeleteQueueItem handles the deletion of a queue item with specified options
func (h *RadarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "radarr", "Radarr")
	if !ok {
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue item id is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.RadarrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceId, queueId, options)
	})

	if handleQueueDeleteError(c, err, "Radarr", instanceId, queueId) {
		return
	}

	h.poller.Refresh(instanceId)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *RadarrHandler) deleteQueueItem(ctx context.Context, instanceId, queueId string, options types.RadarrQueueDeleteOptions) error {
	radarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "radarr")
	if err != nil {
		return err
	}

	// Create Radarr service instance
	service := &radarr.RadarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(ctx, radarrConfig.URL, radarrConfig.APIKey, queueId, options)
}
