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
	authLookupTimeout            = 5 * time.Second

	// SessionCookieName is prefixed with the app name so other services on the
	// same host cannot clobber it (cookies are scoped per host, not per port).
	SessionCookieName = "dashbrr_user_session"
)

type AuthMiddleware struct {
	cache cache.Store
}

func NewAuthMiddleware(cache cache.Store) *AuthMiddleware {
	return &AuthMiddleware{
		cache: cache,
	}
}

func attachAuthContext(c *gin.Context, baseCtx context.Context, sessionData types.SessionData) {
	newCtx := context.WithValue(baseCtx, SessionContextKey, sessionData)
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

func bearerTokenFromHeader(authHeader string) (string, bool) {
	if authHeader == "" {
		return "", false
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}

	return parts[1], true
}

func (m *AuthMiddleware) loadSession(ctx context.Context, sessionToken string) (types.SessionData, error) {
	var sessionData types.SessionData

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionToken)
	if err := m.cache.Get(ctx, sessionKey, &sessionData); err == nil {
		return sessionData, nil
	} else if err != cache.ErrKeyNotFound {
		// Don't mask upstream/cache failures as anonymous misses.
		return types.SessionData{}, err
	}

	sessionKey = fmt.Sprintf("session:%s", sessionToken)
	if err := m.cache.Get(ctx, sessionKey, &sessionData); err != nil {
		return types.SessionData{}, err
	}

	return sessionData, nil
}

// RequireAuth middleware checks for valid authentication
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		baseCtx := c.Request.Context()
		if IsAuthBypassEnabled() {
			attachAuthContext(c, baseCtx, bypassSessionData())
			c.Next()
			return
		}
		lookupCtx, cancel := context.WithTimeout(baseCtx, authLookupTimeout)
		defer cancel()

		// Get session cookie, fallback to Authorization header.
		sessionToken, err := c.Cookie(SessionCookieName)
		if err != nil {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "No authentication provided"})
				c.Abort()
				return
			}

			token, ok := bearerTokenFromHeader(authHeader)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
				c.Abort()
				return
			}
			sessionToken = token
		}

		sessionData, err := m.loadSession(lookupCtx, sessionToken)
		if err != nil {
			if lookupCtx.Err() != nil {
				log.Error().Err(lookupCtx.Err()).Msg("Context cancelled while checking session")
				c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Authentication check timed out"})
				c.Abort()
				return
			}
			if err != cache.ErrKeyNotFound {
				log.Error().Err(err).Msg("error checking session in cache")
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service unavailable"})
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}

		// Attach auth metadata to the original request context.
		// Do not propagate the short lookup timeout to downstream handlers (e.g. SSE streams).
		attachAuthContext(c, baseCtx, sessionData)

		c.Next()
	}
}

// OptionalAuth middleware checks for authentication but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		baseCtx := c.Request.Context()
		if IsAuthBypassEnabled() {
			attachAuthContext(c, baseCtx, bypassSessionData())
			c.Next()
			return
		}
		lookupCtx, cancel := context.WithTimeout(baseCtx, authLookupTimeout)
		defer cancel()

		sessionToken, err := c.Cookie(SessionCookieName)
		if err != nil {
			authHeader := c.GetHeader("Authorization")
			token, ok := bearerTokenFromHeader(authHeader)
			if !ok {
				c.Next()
				return
			}
			sessionToken = token
		}

		if sessionToken == "" {
			c.Next()
			return
		}

		sessionData, err := m.loadSession(lookupCtx, sessionToken)
		if err != nil {
			if lookupCtx.Err() != nil {
				log.Debug().Err(lookupCtx.Err()).Msg("Context cancelled while checking optional session")
				c.Next()
				return
			}
			c.Next()
			return
		}

		// Attach auth metadata to original request context; avoid leaking short lookup timeout.
		attachAuthContext(c, baseCtx, sessionData)

		c.Next()
	}
}
