// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/services/cache"
)

type RateLimiter struct {
	cache     *cache.Cache
	window    time.Duration
	limit     int
	keyPrefix string
}

// NewRateLimiter creates a new rate limiter with the specified configuration
func NewRateLimiter(cache *cache.Cache, window time.Duration, limit int, keyPrefix string) *RateLimiter {
	if window == 0 {
		window = time.Hour
	}
	if limit == 0 {
		limit = 1000
	}
	return &RateLimiter{
		cache:     cache,
		window:    window,
		limit:     limit,
		keyPrefix: keyPrefix,
	}
}

// RateLimit returns a Gin middleware function that implements rate limiting
func (rl *RateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		if clientIP == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not determine client IP"})
			c.Abort()
			return
		}

		// Create key for this IP and endpoint
		endpoint := c.Request.URL.Path
		key := fmt.Sprintf("%s%s:%s", rl.keyPrefix, endpoint, clientIP)
		now := time.Now().Unix()
		windowStart := now - int64(rl.window.Seconds())

		// Clean up old requests
		if err := rl.cache.CleanAndCount(c, key, windowStart); err != nil {
			log.Error().Err(err).Msg("Failed to clean rate limit data")
			c.Next() // Continue on error
			return
		}

		// Get current count
		count, err := rl.cache.GetCount(c, key)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get rate limit count")
			c.Next() // Continue on error
			return
		}

		// Check if limit exceeded
		if count >= int64(rl.limit) {
			retryAfter := windowStart + int64(rl.window.Seconds()) - now
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", windowStart+int64(rl.window.Seconds())))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"limit":       rl.limit,
				"window":      rl.window.String(),
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		// Record this request
		if err := rl.cache.Increment(c, key, now); err != nil {
			log.Error().Err(err).Msg("Failed to record request")
			c.Next() // Continue on error
			return
		}

		// Set expiration
		if err := rl.cache.Expire(c, key, rl.window); err != nil {
			log.Error().Err(err).Msg("Failed to set expiration")
		}

		// Set rate limit headers
		remaining := rl.limit - int(count) - 1
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", windowStart+int64(rl.window.Seconds())))

		c.Next()
	}
}
