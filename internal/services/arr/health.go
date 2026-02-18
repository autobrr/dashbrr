// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

const (
	updateCacheTTL      = time.Hour
	updateErrorCacheTTL = 10 * time.Minute
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
func ArrHealthCheck(ctx context.Context, s *core.ServiceCore, url, apiKey string, checker HealthChecker) (models.ServiceHealth, int) {
	if url == "" {
		return s.CreateHealthResponse(time.Now(), "error", "URL is required"), http.StatusBadRequest
	}

	startTime := time.Now()
	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	health, err := performHealthCheck(healthCtx, s, url, apiKey, checker)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Health check failed")
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Health check failed: %v", err)), http.StatusOK
	}

	return health, http.StatusOK
}

// performHealthCheck executes the actual health check
func performHealthCheck(ctx context.Context, s *core.ServiceCore, url, apiKey string, checker HealthChecker) (models.ServiceHealth, error) {
	startTime := time.Now()

	// Get version synchronously first
	version := s.GetVersionFromCache(ctx, url)
	if version == "" {
		var err error
		version, err = checker.GetSystemStatus(ctx, url, apiKey)
		if err == nil {
			s.CacheVersion(ctx, url, version, time.Hour)
		}
	}

	// Make health check request
	healthEndpoint := checker.GetHealthEndpoint(url)
	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	updateAvailable, hasCachedUpdate := s.GetUpdateStatusFromCacheWithFound(ctx, url)
	if ctx.Err() == nil && !hasCachedUpdate {
		go func() {
			updateBaseCtx := context.WithoutCancel(ctx)
			updateCtx, cancel := context.WithTimeout(updateBaseCtx, core.DefaultTimeout)
			defer cancel()
			cacheCtx, cacheCancel := context.WithTimeout(updateBaseCtx, core.DefaultTimeout)
			defer cacheCancel()

			hasUpdate, err := checker.CheckForUpdates(updateCtx, url, apiKey)
			if err != nil {
				cancelledErr := errors.Is(err, context.Canceled) ||
					errors.Is(err, context.DeadlineExceeded) ||
					errors.Is(err, core.ErrContextCanceled)
				if !cancelledErr {
					log.Debug().Err(err).Str("url", url).Msg("Update check failed")
				}
				if err := s.CacheUpdateStatus(cacheCtx, url, updateAvailable, updateErrorCacheTTL); err != nil {
					log.Debug().Err(err).Str("url", url).Msg("Failed to cache update status")
				}
				return
			}

			if err := s.CacheUpdateStatus(cacheCtx, url, hasUpdate, updateCacheTTL); err != nil {
				log.Debug().Err(err).Str("url", url).Msg("Failed to cache update status")
			}
		}()
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, healthEndpoint, headers, nil)
	if err != nil {
		return models.ServiceHealth{}, fmt.Errorf("failed to connect: %v", err)
	}
	if resp == nil {
		return models.ServiceHealth{}, fmt.Errorf("nil response")
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return models.ServiceHealth{}, fmt.Errorf("failed to read response: %v", err)
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
		return models.ServiceHealth{}, fmt.Errorf("failed to parse response: %v", err)
	}

	// Build response
	extras := map[string]interface{}{
		"responseTime":    responseTimeMs,
		"updateAvailable": updateAvailable,
	}

	// Set version in extras
	if version != "" {
		extras["version"] = version
	}

	// Determine status and message
	status := "online"
	var warnings []string
	seenWarnings := make(map[string]struct{}, len(healthIssues))
	for _, issue := range healthIssues {
		if issue.Type == "warning" || issue.Type == "error" {
			warning := fmt.Sprintf("[%s] %s", issue.Source, issue.Message)
			if _, seen := seenWarnings[warning]; seen {
				continue
			}
			seenWarnings[warning] = struct{}{}
			warnings = append(warnings, warning)
			status = "warning"
		}
	}

	message := "Healthy"
	if len(warnings) > 0 {
		message = strings.Join(warnings, "\n\n")
	}

	health := s.CreateHealthResponse(startTime, status, message, extras)

	return health, nil
}
