// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/arr"
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
