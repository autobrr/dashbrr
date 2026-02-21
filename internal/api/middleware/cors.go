// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupCORS returns the CORS middleware configuration
func SetupCORS(allowedOrigins, allowedHeaders, allowedMethods []string, maxAge time.Duration, allowCredentials *bool) gin.HandlerFunc {
	// Defaults: permissive for same-origin / reverse-proxy setups.
	if len(allowedMethods) == 0 {
		allowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{"Origin", "Authorization", "Content-Type", "Accept", "X-Requested-With"}
	}
	if maxAge <= 0 {
		maxAge = 12 * time.Hour
	}

	config := cors.Config{
		AllowMethods:  allowedMethods,
		AllowHeaders:  allowedHeaders,
		ExposeHeaders: []string{"Content-Length", "Content-Type"},
		MaxAge:        maxAge,
	}

	// If you want credentialed cross-origin requests (cookies for browser auth / SSE),
	// you must NOT use "*" for origins. Use an explicit allowlist.
	if len(allowedOrigins) == 0 {
		config.AllowAllOrigins = true
	} else {
		allowAll := false
		for _, o := range allowedOrigins {
			if strings.TrimSpace(o) == "*" {
				allowAll = true
				break
			}
		}
		if allowAll {
			config.AllowAllOrigins = true
		} else {
			config.AllowOrigins = allowedOrigins
			// Default to enabling credentials when an explicit allowlist is set.
			if allowCredentials != nil {
				config.AllowCredentials = *allowCredentials
			} else {
				config.AllowCredentials = true
			}
		}
	}

	return cors.New(config)
}
