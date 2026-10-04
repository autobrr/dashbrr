// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/session"
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
	sessions *session.Manager
}

func NewAuthMiddleware(sessions *session.Manager) *AuthMiddleware {
	return &AuthMiddleware{
		sessions: sessions,
	}
}

func attachAuthContext(c *gin.Context, sessionData types.SessionData) {
	newCtx := context.WithValue(c.Request.Context(), SessionContextKey, sessionData)
	newCtx = context.WithValue(newCtx, AuthTypeKey, sessionData.AuthType)
	if sessionData.UserID != 0 {
		newCtx = context.WithValue(newCtx, UserIDKey, sessionData.UserID)
	}

	c.Request = c.Request.WithContext(newCtx)

	c.Set("session", sessionData)
	c.Set("auth_type", sessionData.AuthType)
	if sessionData.UserID != 0 {
		c.Set("user_id", sessionData.UserID)
	}
}

func bypassSessionData() types.SessionData {
	return types.SessionData{
		AuthType: "builtin",
		UserID:   1,
	}
}

// AbortWithSessionError writes the response for an error from session.Manager.Load.
func AbortWithSessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, session.ErrNoToken):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authentication provided"})
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		log.Error().Err(err).Msg("Context cancelled while checking session")
		c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "Authentication check timed out"})
	case errors.Is(err, cache.ErrKeyNotFound):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
	default:
		log.Error().Err(err).Msg("error checking session in cache")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service unavailable"})
	}
}

// RequireAuth middleware checks for valid authentication
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAuthBypassEnabled() {
			attachAuthContext(c, bypassSessionData())
			c.Next()
			return
		}

		_, sessionData, err := m.sessions.Load(c)
		if err != nil {
			AbortWithSessionError(c, err)
			return
		}

		attachAuthContext(c, sessionData)
		c.Next()
	}
}

// OptionalAuth middleware checks for authentication but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAuthBypassEnabled() {
			attachAuthContext(c, bypassSessionData())
			c.Next()
			return
		}

		if _, sessionData, err := m.sessions.Load(c); err == nil {
			attachAuthContext(c, sessionData)
		}
		c.Next()
	}
}
