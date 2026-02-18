// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func requireInstanceID(c *gin.Context, prefix, serviceName string) (string, bool) {
	instanceID := c.Query("instanceId")
	if instanceID == "" {
		log.Error().Str("service", serviceName).Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return "", false
	}

	if prefix != "" && !strings.HasPrefix(instanceID, prefix) {
		log.Error().
			Str("service", serviceName).
			Str("instanceId", instanceID).
			Msg("Invalid instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid " + serviceName + " instance ID"})
		return "", false
	}

	return instanceID, true
}
