// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/cache"
)

const (
	HealthCheckTTL = 5 * time.Minute  // 5 minutes for health checks
	DefaultTTL     = 30 * time.Second // 30 seconds default for other endpoints
)

type CacheMiddleware struct {
	cache *cache.Cache
}

type CachedResponse struct {
	Status      int               `json:"status"`
	Body        []byte            `json:"body"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
}

func NewCacheMiddleware(cache *cache.Cache) *CacheMiddleware {
	return &CacheMiddleware{
		cache: cache,
	}
}

func (m *CacheMiddleware) Cache() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Use the full URL path as cache key
		cacheKey := c.Request.URL.String()

		// Try to get from cache
		var cachedResponse CachedResponse
		err := m.cache.Get(c.Request.Context(), cacheKey, &cachedResponse)
		if err == nil {
			// Set cached headers
			for k, v := range cachedResponse.Headers {
				c.Header(k, v)
			}

			c.Header("X-Cache", "HIT")
			c.Data(cachedResponse.Status, cachedResponse.ContentType, cachedResponse.Body)
			c.Abort()
			return
		}

		// Create a buffer to store the response
		w := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
			headers:        make(map[string]string),
		}

		// Replace writer
		c.Writer = w

		// Process request
		c.Next()

		// Only cache successful JSON responses
		contentType := w.Header().Get("Content-Type")
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 && isJSONResponse(contentType) {
			// Store headers
			headers := make(map[string]string)
			for k, v := range w.Header() {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			responseData := CachedResponse{
				Status:      c.Writer.Status(),
				Body:        w.body.Bytes(),
				ContentType: contentType,
				Headers:     headers,
			}

			// Determine TTL based on endpoint
			ttl := m.getTTL(c.Request.URL.Path)

			err := m.cache.Set(c.Request.Context(), cacheKey, responseData, ttl)
			if err != nil {
				log.Error().Err(err).Str("key", cacheKey).Msg("Failed to cache response")
			}
		}

		// Set cache status header
		c.Header("X-Cache", "MISS")
	}
}

// getTTL determines cache TTL based on the endpoint
func (m *CacheMiddleware) getTTL(path string) time.Duration {
	// Health check endpoints
	if strings.Contains(path, "/health") {
		return HealthCheckTTL
	}

	// Service-specific TTLs
	switch {
	case strings.Contains(path, "/plex/sessions"):
		return 10 * time.Second
	case strings.Contains(path, "/autobrr"):
		return 30 * time.Second
	case strings.Contains(path, "/overseerr"):
		return 1 * time.Minute
	case strings.Contains(path, "/maintainerr"):
		return 1 * time.Minute
	case strings.Contains(path, "/sonarr"):
		return 30 * time.Second
	case strings.Contains(path, "/radarr"):
		return 30 * time.Second
	case strings.Contains(path, "/prowlarr"):
		return 1 * time.Minute
	default:
		return DefaultTTL
	}
}

func isJSONResponse(contentType string) bool {
	return contentType == "application/json" || contentType == "application/json; charset=utf-8"
}

type responseWriter struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	headers map[string]string
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *responseWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
}
