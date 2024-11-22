// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
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
	GetSystemStatus(ctx context.Context, url, apiKey string) (string, error)
	CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error)
	GetHealthEndpoint(baseURL string) string
}

// ArrHealthCheck provides a common implementation of health checking for *arr services
func ArrHealthCheck(ctx context.Context, s *ServiceCore, checker HealthChecker) (domain.ServiceHealth, int) {
	//log.Debug().Str("service", "arr").Str("url", s.URL).Str("name", s.DisplayName).Msg("Performing arr health check")

	if s.URL == "" {
		return s.CreateHealthResponse(time.Now(), "error", "URL is required"), http.StatusBadRequest
	}

	// FIXME this calls functions that call each other in a loop?

	// Try to get cached health response
	cacheKey := arrCachePrefix + "health:" + s.InstanceID
	//var cachedHealth domain.ServiceHealth
	//if _, err := s.GetCachedVersion(ctx, cacheKey, "", func(_, _ string) (string, error) {
	//	return "", nil // Cache miss, will handle below
	//}); err == nil && cachedHealth.Status != "" {
	//	// Refresh cache in background
	//	go func() {
	//		refreshKey := fmt.Sprintf("refresh:%s", url)
	//		_, _, _ = sf.Do(refreshKey, func() (interface{}, error) {
	//			return performHealthCheck(ctx, s, url, apiKey, checker)
	//		})
	//	}()
	//	return cachedHealth, http.StatusOK
	//}
	log.Debug().Str("service", "arr").Str("url", s.URL).Str("cacheKey", cacheKey).Str("name", s.DisplayName).Msg("Performing arr health check")

	startTime := time.Now()

	//// Use singleflight for health check
	//healthKey := fmt.Sprintf("health:%s", url)
	result, err, _ := sf.Do(cacheKey, func() (interface{}, error) {
		return performHealthCheck(ctx, s, cacheKey, checker)
	})

	if err != nil {
		log.Error().Err(err).Str("url", s.URL).Msg("Health check failed")
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Health check failed: %v", err)), http.StatusOK
	}

	health := result.(domain.ServiceHealth)
	return health, http.StatusOK
}

// performHealthCheck executes the actual health check
func performHealthCheck(ctx context.Context, s *ServiceCore, cacheKey string, checker HealthChecker) (domain.ServiceHealth, error) {
	log.Debug().Str("url", s.URL).Str("instance", s.InstanceID).Msg("Performing health check")

	startTime := time.Now()

	// Get version synchronously first
	version := s.GetVersionFromCache(s.URL)
	if version == "" {
		var err error
		version, err = checker.GetSystemStatus(ctx, s.URL, s.ApiKey)
		if err == nil {
			s.CacheVersion(nil, s.URL, version, time.Hour)
		}
	}

	// Make health check request
	healthEndpoint := checker.GetHealthEndpoint(s.URL)
	headers := map[string]string{
		"X-Api-Key": s.ApiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, s.ApiKey, headers)
	if err != nil {
		return domain.ServiceHealth{}, fmt.Errorf("failed to connect: %v", err)
	}
	if resp == nil {
		return domain.ServiceHealth{}, fmt.Errorf("nil response")
	}

	defer resp.Body.Close()
	body, err := s.ReadBody(resp)
	if err != nil {
		return domain.ServiceHealth{}, fmt.Errorf("failed to read response: %v", err)
	}

	// Get response time from header (stored as milliseconds)
	var responseTimeMs int64
	if respTimeStr := resp.Header.Get("X-Response-Time"); respTimeStr != "" {
		if ms, err := strconv.ParseInt(respTimeStr, 10, 64); err == nil {
			responseTimeMs = ms
		}
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
		return domain.ServiceHealth{}, fmt.Errorf("failed to parse response: %v", err)
	}

	// Build response
	extras := map[string]interface{}{
		"responseTime": responseTimeMs,
	}

	// Set version in extras
	if version != "" {
		extras["version"] = version
	}

	// Check for updates in background
	go func() {
		// TODO this should be somewheere else
		log.Trace().Msg("arrHealthcheck: check for updates in the background")

		if hasUpdate, err := checker.CheckForUpdates(ctx, s.URL, s.ApiKey); err == nil && hasUpdate {
			updateKey := fmt.Sprintf("%s:update", s.URL)
			s.CacheVersion(ctx, updateKey, "true", time.Hour)
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
		//cacheKey := arrCachePrefix + "health:" + s.InstanceID
		if err := s.CacheHealth(ctx, cacheKey, fmt.Sprintf("%+v", health), healthCacheDuration); err != nil {
			log.Warn().Err(err).Str("url", s.URL).Msg("Failed to cache health response")
		}
	}

	return health, nil
}
