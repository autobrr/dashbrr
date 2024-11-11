// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/config"
)

// Config middleware injects the application config into the Gin context
func Config(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("config", cfg)
		c.Next()
	}
}
