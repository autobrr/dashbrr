// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

const (
	healthCacheDuration = 30 * time.Second
	arrCachePrefix      = "arr:"
)

var (
	sf singleflight.Group
	mu sync.RWMutex
)

// HealthResponse represents a common health check response structure
type HealthResponse struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

// HealthChecker interface defines methods required for health checking
type HealthChecker interface {
	GetSystemStatus(url, apiKey string) (string, error)
	CheckForUpdates(url, apiKey string) (bool, error)
	GetHealthEndpoint(baseURL string) string
}

// ArrHealthCheck provides a common implementation of health checking for *arr services
func ArrHealthCheck(s *core.ServiceCore, url, apiKey string, checker HealthChecker) (models.ServiceHealth, int) {
	if url == "" {
		return s.CreateHealthResponse(time.Now(), "error", "URL is required"), http.StatusBadRequest
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	// Try to get cached health response
	cacheKey := arrCachePrefix + "health:" + url
	var cachedHealth models.ServiceHealth
	if _, err := s.GetCachedVersion(ctx, cacheKey, "", func(_, _ string) (string, error) {
		return "", nil // Cache miss, will handle below
	}); err == nil && cachedHealth.Status != "" {
		// Refresh cache in background
		go func() {
			refreshKey := fmt.Sprintf("refresh:%s", url)
			_, _, _ = sf.Do(refreshKey, func() (interface{}, error) {
				return performHealthCheck(ctx, s, url, apiKey, checker)
			})
		}()
		return cachedHealth, http.StatusOK
	}

	// Use singleflight for health check
	healthKey := fmt.Sprintf("health:%s", url)
	result, err, _ := sf.Do(healthKey, func() (interface{}, error) {
		return performHealthCheck(ctx, s, url, apiKey, checker)
	})

	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Health check failed")
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Health check failed: %v", err)), http.StatusOK
	}

	health := result.(models.ServiceHealth)
	return health, http.StatusOK
}

// performHealthCheck executes the actual health check
func performHealthCheck(ctx context.Context, s *core.ServiceCore, url, apiKey string, checker HealthChecker) (models.ServiceHealth, error) {
	startTime := time.Now()

	// Make health check request
	healthEndpoint := checker.GetHealthEndpoint(url)
	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, apiKey, headers)
	if err != nil {
		return models.ServiceHealth{}, fmt.Errorf("failed to connect: %v", err)
	}
	if resp == nil {
		return models.ServiceHealth{}, fmt.Errorf("nil response")
	}

	defer resp.Body.Close()
	body, err := s.ReadBody(resp)
	if err != nil {
		return models.ServiceHealth{}, fmt.Errorf("failed to read response: %v", err)
	}

	// Get response time
	respTimeStr := resp.Header.Get("X-Response-Time")
	var respTime time.Duration
	if respTimeStr != "" {
		respTime, _ = time.ParseDuration(respTimeStr)
	}

	// Handle error status codes
	if resp.StatusCode >= 400 {
		statusText := http.StatusText(resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Service is temporarily unavailable (%d %s)", resp.StatusCode, statusText)), nil
		case http.StatusUnauthorized:
			return s.CreateHealthResponse(startTime, "error", "Invalid API key"), nil
		case http.StatusForbidden:
			return s.CreateHealthResponse(startTime, "error", "Access forbidden"), nil
		case http.StatusNotFound:
			return s.CreateHealthResponse(startTime, "error", "Service endpoint not found"), nil
		default:
			return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned %s (%d)", statusText, resp.StatusCode)), nil
		}
	}

	// Process health response
	var healthIssues []HealthResponse
	if err := json.Unmarshal(body, &healthIssues); err != nil {
		return models.ServiceHealth{}, fmt.Errorf("failed to parse response: %v", err)
	}

	// Build response
	extras := map[string]interface{}{
		"responseTime": respTime.Milliseconds(),
	}

	// Get version and update status in background
	go func() {
		if version := s.GetVersionFromCache(url); version == "" {
			if v, err := checker.GetSystemStatus(url, apiKey); err == nil {
				s.CacheVersion(url, v, time.Hour)
				extras["version"] = v
			}
		}
		if _, err := checker.CheckForUpdates(url, apiKey); err == nil {
			s.CacheVersion(url, "true", time.Hour)
			extras["updateAvailable"] = true
		}
	}()

	// Determine status and message
	status := "online"
	var warnings []string
	for _, issue := range healthIssues {
		if issue.Type == "warning" || issue.Type == "error" {
			warnings = append(warnings, fmt.Sprintf("[%s] %s", issue.Source, issue.Message))
			status = "warning"
		}
	}

	message := "Healthy"
	if len(warnings) > 0 {
		message = strings.Join(warnings, "\n\n")
	}

	health := s.CreateHealthResponse(startTime, status, message, extras)

	// Cache the health response
	if status != "error" {
		cacheKey := arrCachePrefix + "health:" + url
		if err := s.CacheVersion(cacheKey, fmt.Sprintf("%+v", health), healthCacheDuration); err != nil {
			log.Warn().Err(err).Str("url", url).Msg("Failed to cache health response")
		}
	}

	return health, nil
}
