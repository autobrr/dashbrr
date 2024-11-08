// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package prowlarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// ErrProwlarr represents a Prowlarr-specific error
type ErrProwlarr struct {
	Message string
	Err     error
}

func (e *ErrProwlarr) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

type ProwlarrService struct {
	core.ServiceCore
}

type HealthResponse struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

type SystemStatusResponse struct {
	Version string `json:"version"`
}

func init() {
	models.NewProwlarrService = func() models.ServiceHealthChecker {
		return &ProwlarrService{}
	}
}

func (s *ProwlarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/health", baseURL)
}

func (s *ProwlarrService) getSystemStatus(baseURL, apiKey string) (string, error) {
	// Check cache first
	if version := s.GetVersionFromCache(baseURL); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v1/system/status", strings.TrimRight(baseURL, "/"))
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.MakeRequestWithContext(ctx, statusURL, "", headers)
	if err != nil {
		return "", &ErrProwlarr{Message: "Failed to get system status", Err: err}
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrProwlarr{Message: "Failed to read system status response", Err: err}
	}

	var status SystemStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return "", &ErrProwlarr{Message: "Failed to parse system status response", Err: err}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(baseURL, status.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return status.Version, nil
}

func (s *ProwlarrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	// Create a context with timeout for the entire health check
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start version check in background
	versionChan := make(chan string, 1)
	go func() {
		version, err := s.getSystemStatus(url, apiKey)
		if err != nil {
			versionChan <- ""
			return
		}
		versionChan <- version
	}()

	// Perform health check
	healthEndpoint := s.GetHealthEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Health check failed: %v", err)), http.StatusServiceUnavailable
	}
	defer resp.Body.Close()

	// Get response time from header
	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to read response: %v", err)), http.StatusInternalServerError
	}

	var healthIssues []HealthResponse
	if err := json.Unmarshal(body, &healthIssues); err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to parse response: %v", err)), http.StatusInternalServerError
	}

	// Wait for version with timeout
	var version string
	select {
	case v := <-versionChan:
		version = v
	case <-time.After(500 * time.Millisecond):
		// Continue without version if it takes too long
	}

	extras := map[string]interface{}{
		"responseTime": responseTime.Milliseconds(),
	}
	if version != "" {
		extras["version"] = version
	}

	var allWarnings []string
	var indexerWarnings []string
	var otherWarnings []string

	// Process health issues
	for _, issue := range healthIssues {
		message := issue.Message
		message = strings.TrimPrefix(message, "IndexerStatusCheck: ")
		message = strings.TrimPrefix(message, "ApplicationLongTermStatusCheck: ")

		// Check for update message
		if strings.HasPrefix(message, "New update is available:") {
			extras["updateAvailable"] = true
			continue
		}

		if strings.Contains(message, "Indexers unavailable due to failures") {
			// Extract indexer names from the message
			parts := strings.Split(message, ":")
			if len(parts) > 1 {
				indexers := strings.Split(parts[1], ",")
				for _, indexer := range indexers {
					indexer = strings.TrimSpace(indexer)
					if indexer != "" {
						indexerWarnings = append(indexerWarnings, fmt.Sprintf("- %s", indexer))
					}
				}
			}
		} else {
			otherWarnings = append(otherWarnings, fmt.Sprintf("- %s", message))
		}
	}

	// Format warnings
	if len(indexerWarnings) > 0 {
		allWarnings = append(allWarnings, fmt.Sprintf("Indexers unavailable due to failures:\n%s", strings.Join(indexerWarnings, "\n")))
	}
	if len(otherWarnings) > 0 {
		allWarnings = append(allWarnings, strings.Join(otherWarnings, "\n"))
	}

	// If there are any warnings, return them all
	if len(allWarnings) > 0 {
		return s.CreateHealthResponse(startTime, "warning", strings.Join(allWarnings, "\n\n"), extras), http.StatusOK
	}

	// If no warnings, the service is healthy
	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}
