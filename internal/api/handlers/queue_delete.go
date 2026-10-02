// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/resilience"
)

// ArrQueueHandler deletes queue items for every *arr app.
type ArrQueueHandler struct {
	db     *database.DB
	poller *Poller
}

func NewArrQueueHandler(db *database.DB, poller *Poller) *ArrQueueHandler {
	return &ArrQueueHandler{db: db, poller: poller}
}

func (h *ArrQueueHandler) DeleteQueueItem(c *gin.Context) {
	instanceID, ok := requireInstanceID(c, "", "*arr")
	if !ok {
		return
	}
	queueID := c.Param("id")
	ctx := c.Request.Context()

	cfg, err := requireServiceConfig(ctx, h.db, instanceID, instanceID)
	if handleQueueDeleteError(c, err, "*arr", instanceID, queueID) {
		return
	}

	serviceType, _ := models.ServiceTypeFromInstanceID(instanceID)
	app, ok := arr.AppFor(serviceType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance is not an *arr app"})
		return
	}

	options := queueDeleteOptionsFromQuery(c)
	err = resilience.RetryWithBackoff(ctx, func() error {
		return app.DeleteQueueItem(ctx, cfg.URL, cfg.APIKey, queueID, options)
	})
	if handleQueueDeleteError(c, err, app.Name, instanceID, queueID) {
		return
	}

	h.poller.Refresh(instanceID)

	c.JSON(http.StatusOK, gin.H{"message": "Queue item deleted successfully"})
}

func queueDeleteOptionsFromQuery(c *gin.Context) arr.QueueDeleteOptions {
	return arr.QueueDeleteOptions{
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

	if arrErr, ok := errors.AsType[*arr.ErrArr](err); ok {
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
