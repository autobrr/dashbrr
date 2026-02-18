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

func handleArrFetchError(c *gin.Context, err error, serviceName, instanceId, operation string) bool {
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
			Str("instanceId", instanceId).
			Msgf("[%s] Failed to fetch %s", serviceName, operation)

		if arrErr.HttpCode > 0 {
			c.JSON(normalizeUpstreamStatus(arrErr.HttpCode), gin.H{"error": arrErr.Error()})
			return true
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": fmt.Sprintf("Failed to fetch %s: %v", operation, err),
	})
	return true
}
