// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

// AuthConfig returns a handler that reports the available authentication methods.
func AuthConfig(hasOIDC bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsAuthBypassEnabled() {
			c.JSON(http.StatusOK, gin.H{
				"methods": map[string]bool{
					"builtin": true,
					"oidc":    false,
				},
				"default": "builtin",
				"bypass":  true,
			})
			return
		}

		defaultMethod := "builtin"
		if hasOIDC {
			defaultMethod = "oidc"
		}

		c.JSON(http.StatusOK, gin.H{
			"methods": map[string]bool{
				"builtin": !hasOIDC, // Built-in auth is only available when OIDC is not configured
				"oidc":    hasOIDC,
			},
			"default": defaultMethod,
			"bypass":  false,
		})
	}
}
