// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/types"
)

type SonarrHandler struct {
	db     *database.DB
	poller *Poller
}

func NewSonarrHandler(db *database.DB, poller *Poller) *SonarrHandler {
	return &SonarrHandler{db: db, poller: poller}
}

func (h *SonarrHandler) DeleteQueueItem(c *gin.Context) {
	instanceId, ok := requireInstanceID(c, "sonarr", "Sonarr")
	if !ok {
		return
	}

	queueId := c.Param("id")
	if queueId == "" {
		log.Error().Msg("No queue ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue ID is required"})
		return
	}

	queryOptions := queueDeleteOptionsFromQuery(c)
	options := types.SonarrQueueDeleteOptions{
		RemoveFromClient: queryOptions.RemoveFromClient,
		Blocklist:        queryOptions.Blocklist,
		SkipRedownload:   queryOptions.SkipRedownload,
		ChangeCategory:   queryOptions.ChangeCategory,
	}

	ctx := c.Request.Context()

	err := resilience.RetryWithBackoff(ctx, func() error {
		return h.deleteQueueItem(ctx, instanceId, queueId, options)
	})

	if handleQueueDeleteError(c, err, "Sonarr", instanceId, queueId) {
		return
	}

	h.poller.Refresh(instanceId)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func (h *SonarrHandler) deleteQueueItem(ctx context.Context, instanceId, queueId string, options types.SonarrQueueDeleteOptions) error {
	sonarrConfig, err := requireServiceConfig(ctx, h.db, instanceId, "sonarr")
	if err != nil {
		return err
	}

	// Create Sonarr service instance
	service := &sonarr.SonarrService{}

	// Call the service method to delete the queue item
	return service.DeleteQueueItem(ctx, sonarrConfig.URL, sonarrConfig.APIKey, queueId, options)
}
