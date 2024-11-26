// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupCORS returns the CORS middleware configuration
func SetupCORS() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Authorization",
			"Content-Type",
			"Accept",
			"X-Requested-With",
			"X-CSRF-Token",
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Headers",
			"Access-Control-Allow-Methods",
			"Access-Control-Allow-Credentials",
		},
		AllowCredentials: true, // Important for OAuth flows
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"X-CSRF-Token",
		},
		MaxAge: 12 * time.Hour,
	}

	return cors.New(config)
}
