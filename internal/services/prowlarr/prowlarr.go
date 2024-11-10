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

// Custom error types for better error handling
type ErrProwlarr struct {
	Op       string // Operation that failed
	Err      error  // Underlying error
	HttpCode int    // HTTP status code if applicable
}

func (e *ErrProwlarr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("prowlarr %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("prowlarr %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("prowlarr %s", e.Op)
}

func (e *ErrProwlarr) Unwrap() error {
	return e.Err
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
	models.NewProwlarrService = NewProwlarrService
}

func NewProwlarrService() models.ServiceHealthChecker {
	service := &ProwlarrService{}
	service.Type = "prowlarr"
	service.DisplayName = "Prowlarr"
	service.Description = "Monitor and manage your Prowlarr instance"
	service.DefaultURL = "http://localhost:9696"
	service.HealthEndpoint = "/api/v1/health"
	return service
}

func (s *ProwlarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/health", baseURL)
}

// makeRequest is a helper function to make requests with proper headers
func (s *ProwlarrService) makeRequest(ctx context.Context, method, url, apiKey string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers correctly
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "*/*")

	client := &http.Client{}
	return client.Do(req)
}

// GetSystemStatus fetches the system status from Prowlarr
func (s *ProwlarrService) GetSystemStatus(baseURL, apiKey string) (string, error) {
	if baseURL == "" {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first
	if version := s.GetVersionFromCache(baseURL); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v1/system/status", strings.TrimRight(baseURL, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, statusURL, apiKey)
	if err != nil {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrProwlarr{Op: "get_system_status", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var status SystemStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
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
	versionErrChan := make(chan error, 1)
	go func() {
		version, err := s.GetSystemStatus(url, apiKey)
		versionChan <- version
		versionErrChan <- err
	}()

	// Perform health check
	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.makeRequest(ctx, http.MethodGet, healthEndpoint, apiKey)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	// Get response time from header
	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to read response: %v", err)), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		statusText := http.StatusText(resp.StatusCode)
		status := "error"
		message := fmt.Sprintf("Server returned %s (%d)", statusText, resp.StatusCode)

		// Determine appropriate status based on response code
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			message = fmt.Sprintf("Service is temporarily unavailable (%d %s)", resp.StatusCode, statusText)
		case http.StatusUnauthorized:
			message = "Invalid API key"
		case http.StatusForbidden:
			message = "Access forbidden"
		case http.StatusNotFound:
			message = "Service endpoint not found"
		}

		return s.CreateHealthResponse(startTime, status, message), http.StatusOK
	}

	var healthIssues []HealthResponse
	if err := json.Unmarshal(body, &healthIssues); err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to parse response: %v", err)), http.StatusOK
	}

	// Wait for version with timeout
	var version string
	var versionErr error
	select {
	case version = <-versionChan:
		versionErr = <-versionErrChan
	case <-time.After(500 * time.Millisecond):
		// Continue without version if it takes too long
	}

	extras := map[string]interface{}{
		"responseTime": responseTime.Milliseconds(),
	}

	if version != "" {
		extras["version"] = version
	}
	if versionErr != nil {
		extras["versionError"] = versionErr.Error()
	}

	var allWarnings []string

	// Process health issues
	for _, issue := range healthIssues {
		if issue.Type == "warning" || issue.Type == "error" {
			message := issue.Message

			// Check for update message
			if strings.HasPrefix(message, "New update is available:") {
				extras["updateAvailable"] = true
				continue
			}

			if strings.Contains(message, "Indexers unavailable due to failures") {
				// Extract indexer names from the message and clean them up
				parts := strings.Split(message, ":")
				if len(parts) > 1 {
					indexers := strings.Split(strings.TrimSpace(parts[1]), ",")
					var indexerWarnings []string
					for _, indexer := range indexers {
						indexer = strings.TrimSpace(indexer)
						if indexer != "" {
							// Remove any "Wiki" suffix and clean up the format
							indexer = strings.TrimSuffix(indexer, " Wiki")
							indexerWarnings = append(indexerWarnings, indexer)
						}
					}
					if len(indexerWarnings) > 0 {
						allWarnings = append(allWarnings, fmt.Sprintf("Indexers unavailable due to failures:\n%s",
							strings.Join(indexerWarnings, "\n")))
					}
				}
			} else {
				warning := message
				if issue.WikiURL != "" {
					warning += fmt.Sprintf("\nWiki: %s", issue.WikiURL)
				}
				if issue.Source != "" &&
					issue.Source != "IndexerLongTermStatusCheck" {
					warning = fmt.Sprintf("[%s] %s", issue.Source, warning)
				}
				allWarnings = append(allWarnings, warning)
			}
		}
	}

	// If there are any warnings, return them all
	if len(allWarnings) > 0 {
		return s.CreateHealthResponse(startTime, "warning", strings.Join(allWarnings, "\n\n"), extras), http.StatusOK
	}

	// If no warnings, the service is healthy
	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}
