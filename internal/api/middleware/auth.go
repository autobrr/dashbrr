// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

type AuthMiddleware struct {
	cache cache.Store
}

func NewAuthMiddleware(cache cache.Store) *AuthMiddleware {
	return &AuthMiddleware{
		cache: cache,
	}
}

// RequireAuth middleware checks for valid authentication
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session cookie
		sessionToken, err := c.Cookie("session")
		if err != nil {
			// Check for Authorization header as fallback
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "No authentication provided"})
				c.Abort()
				return
			}

			// Extract token from Authorization header
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
				c.Abort()
				return
			}
			sessionToken = parts[1]
		}

		// Check session in Redis
		var sessionKey string
		var sessionData types.SessionData

		// Try OIDC session format first
		sessionKey = fmt.Sprintf("oidc:session:%s", sessionToken)
		err = m.cache.Get(c, sessionKey, &sessionData)
		if err != nil {
			// If not found, try built-in auth session format
			sessionKey = fmt.Sprintf("session:%s", sessionToken)
			err = m.cache.Get(c, sessionKey, &sessionData)
			if err != nil {
				log.Error().Err(err).Msg("session not found")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
				c.Abort()
				return
			}
		}

		// Store session data in context
		c.Set("session", sessionData)
		c.Set("auth_type", sessionData.AuthType)
		if sessionData.UserID != 0 {
			c.Set("user_id", sessionData.UserID)
		}

		c.Next()
	}
}

// OptionalAuth middleware checks for authentication but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken, err := c.Cookie("session")
		if err != nil {
			c.Next()
			return
		}

		var sessionKey string
		var sessionData types.SessionData

		// Try OIDC session format first
		sessionKey = fmt.Sprintf("oidc:session:%s", sessionToken)
		err = m.cache.Get(c, sessionKey, &sessionData)
		if err != nil {
			// If not found, try built-in auth session format
			sessionKey = fmt.Sprintf("session:%s", sessionToken)
			err = m.cache.Get(c, sessionKey, &sessionData)
			if err != nil {
				c.Next()
				return
			}
		}

		// Store session data in context
		c.Set("session", sessionData)
		c.Set("auth_type", sessionData.AuthType)
		if sessionData.UserID != 0 {
			c.Set("user_id", sessionData.UserID)
		}

		c.Next()
	}
}
