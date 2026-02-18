// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

// Custom context keys
type contextKey string

const (
	SessionContextKey contextKey = "session_data"
	AuthTypeKey       contextKey = "auth_type"
	UserIDKey         contextKey = "user_id"
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
		baseCtx := c.Request.Context()
		lookupCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
		defer cancel()

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
		err = m.cache.Get(lookupCtx, sessionKey, &sessionData)
		if err != nil {
			// If not found, try built-in auth session format
			sessionKey = fmt.Sprintf("session:%s", sessionToken)
			err = m.cache.Get(lookupCtx, sessionKey, &sessionData)
			if err != nil {
				// Check for context cancellation
				if lookupCtx.Err() != nil {
					log.Error().Err(lookupCtx.Err()).Msg("Context cancelled while checking session")
					c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Authentication check timed out"})
					c.Abort()
					return
				}
				// Only log if it's not a "key not found" error, as that's expected
				if err != cache.ErrKeyNotFound {
					log.Error().Err(err).Msg("error checking session in cache")
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
				c.Abort()
				return
			}
		}

		// Attach auth metadata to the original request context.
		// Do not propagate the short lookup timeout to downstream handlers (e.g. SSE streams).
		newCtx := context.WithValue(baseCtx, SessionContextKey, sessionData)
		newCtx = context.WithValue(newCtx, AuthTypeKey, sessionData.AuthType)
		if sessionData.UserID != 0 {
			newCtx = context.WithValue(newCtx, UserIDKey, sessionData.UserID)
		}

		// Update request context
		c.Request = c.Request.WithContext(newCtx)

		// Also set in gin context for backward compatibility
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
		baseCtx := c.Request.Context()
		lookupCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
		defer cancel()

		sessionToken, err := c.Cookie("session")
		if err != nil {
			c.Next()
			return
		}

		var sessionKey string
		var sessionData types.SessionData

		// Try OIDC session format first
		sessionKey = fmt.Sprintf("oidc:session:%s", sessionToken)
		err = m.cache.Get(lookupCtx, sessionKey, &sessionData)
		if err != nil {
			// If not found, try built-in auth session format
			sessionKey = fmt.Sprintf("session:%s", sessionToken)
			err = m.cache.Get(lookupCtx, sessionKey, &sessionData)
			if err != nil {
				// Check for context cancellation
				if lookupCtx.Err() != nil {
					log.Debug().Err(lookupCtx.Err()).Msg("Context cancelled while checking optional session")
					c.Next()
					return
				}
				// Don't log anything for optional auth failures
				c.Next()
				return
			}
		}

		// Attach auth metadata to original request context; avoid leaking short lookup timeout.
		newCtx := context.WithValue(baseCtx, SessionContextKey, sessionData)
		newCtx = context.WithValue(newCtx, AuthTypeKey, sessionData.AuthType)
		if sessionData.UserID != 0 {
			newCtx = context.WithValue(newCtx, UserIDKey, sessionData.UserID)
		}

		// Update request context
		c.Request = c.Request.WithContext(newCtx)

		// Also set in gin context for backward compatibility
		c.Set("session", sessionData)
		c.Set("auth_type", sessionData.AuthType)
		if sessionData.UserID != 0 {
			c.Set("user_id", sessionData.UserID)
		}

		c.Next()
	}
}
