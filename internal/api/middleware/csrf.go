// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	csrfTokenLength   = 32
	csrfTokenHeader   = "X-CSRF-Token"
	csrfTokenCookie   = "csrf_token"
	csrfTokenDuration = 24 * time.Hour
)

var (
	ErrTokenMissing  = errors.New("CSRF token missing")
	ErrTokenMismatch = errors.New("CSRF token mismatch")
)

// CSRFConfig holds configuration for CSRF protection
type CSRFConfig struct {
	// Secure indicates if the cookie should be sent only over HTTPS
	Secure bool
	// Cookie path
	Path string
	// Cookie domain
	Domain string
	// Cookie max age in seconds
	MaxAge int
	// If true, cookie is not accessible via JavaScript
	HttpOnly bool
	// Methods that don't require CSRF validation
	ExemptMethods []string
	// Paths that don't require CSRF validation
	ExemptPaths []string
}

// DefaultCSRFConfig returns the default CSRF configuration
func DefaultCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		Secure:        true,
		Path:          "/",
		HttpOnly:      true,
		MaxAge:        int(csrfTokenDuration.Seconds()),
		ExemptMethods: []string{"GET", "HEAD", "OPTIONS"},
		ExemptPaths:   []string{},
	}
}

// generateCSRFToken generates a random CSRF token
func generateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CSRF returns a middleware that provides CSRF protection
func CSRF(config *CSRFConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultCSRFConfig()
	}

	return func(c *gin.Context) {
		// Check if the path is exempt
		for _, path := range config.ExemptPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Check if the method is exempt
		method := strings.ToUpper(c.Request.Method)
		for _, m := range config.ExemptMethods {
			if method == m {
				// For GET requests, set a new token if one doesn't exist
				if method == "GET" {
					_, err := c.Cookie(csrfTokenCookie)
					if err == http.ErrNoCookie {
						token, err := generateCSRFToken()
						if err != nil {
							c.AbortWithStatus(http.StatusInternalServerError)
							return
						}
						c.SetCookie(csrfTokenCookie, token, config.MaxAge, config.Path,
							config.Domain, config.Secure, config.HttpOnly)
						c.Header(csrfTokenHeader, token)
					}
				}
				c.Next()
				return
			}
		}

		// Get the token from the cookie
		cookie, err := c.Cookie(csrfTokenCookie)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrTokenMissing.Error()})
			c.Abort()
			return
		}

		// Get the token from the header
		header := c.GetHeader(csrfTokenHeader)
		if header == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrTokenMissing.Error()})
			c.Abort()
			return
		}

		// Compare the cookie token with the header token
		if cookie != header {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrTokenMismatch.Error()})
			c.Abort()
			return
		}

		// Generate a new token for the next request
		newToken, err := generateCSRFToken()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// Set the new token in both cookie and header
		c.SetCookie(csrfTokenCookie, newToken, config.MaxAge, config.Path,
			config.Domain, config.Secure, config.HttpOnly)
		c.Header(csrfTokenHeader, newToken)

		c.Next()
	}
}
