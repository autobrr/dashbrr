// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package prowlarr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
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
	service.SetTimeout(core.DefaultTimeout)
	return service
}

// makeRequest is a helper function to make requests with proper headers
func (s *ProwlarrService) makeRequest(ctx context.Context, method, url, apiKey string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "*/*")

	client := &http.Client{}
	return client.Do(req)
}

// GetSystemStatus fetches the system status from Prowlarr
func (s *ProwlarrService) GetSystemStatus(ctx context.Context, baseURL, apiKey string) (string, error) {
	if baseURL == "" {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first
	if version := s.GetVersionFromCache(baseURL); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v1/system/status", strings.TrimRight(baseURL, "/"))
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

// GetIndexerStats fetches indexer statistics from Prowlarr
func (s *ProwlarrService) GetIndexerStats(ctx context.Context, baseURL, apiKey string) (*types.ProwlarrIndexerStatsResponse, error) {
	if baseURL == "" {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("URL is required")}
	}

	statsURL := fmt.Sprintf("%s/api/v1/indexerstats", strings.TrimRight(baseURL, "/"))
	resp, err := s.makeRequest(ctx, http.MethodGet, statsURL, apiKey)
	if err != nil {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var stats types.ProwlarrIndexerStatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &stats, nil
}

// CheckForUpdates checks if there are any updates available
func (s *ProwlarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	// Prowlarr doesn't have a dedicated updates endpoint, updates are reported through health checks
	return false, nil
}

// GetQueue gets the current queue status
func (s *ProwlarrService) GetQueue(ctx context.Context, url, apiKey string) (interface{}, error) {
	// Prowlarr doesn't have a queue system
	return nil, nil
}

// GetHealthEndpoint returns the health endpoint for Prowlarr
func (s *ProwlarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/health", baseURL)
}

func (s *ProwlarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "pending", "Prowlarr not configured"), http.StatusOK
	}

	// Get version
	version, err := s.GetSystemStatus(ctx, url, apiKey)
	if err != nil {
		var prowlarrErr *ErrProwlarr
		if strings.Contains(err.Error(), "connection refused") {
			return s.CreateHealthResponse(startTime, "offline", "Connection refused"), http.StatusOK
		}
		if strings.Contains(err.Error(), "no such host") {
			return s.CreateHealthResponse(startTime, "offline", "Host not found"), http.StatusOK
		}
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return s.CreateHealthResponse(startTime, "timeout", "Request timed out"), http.StatusOK
		}
		if strings.Contains(err.Error(), "certificate") {
			return s.CreateHealthResponse(startTime, "error", "Invalid SSL certificate"), http.StatusOK
		}
		if ok := errors.As(err, &prowlarrErr); ok && prowlarrErr.HttpCode == http.StatusUnauthorized {
			return s.CreateHealthResponse(startTime, "error", "Invalid API key"), http.StatusUnauthorized
		}
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to get version: %v", err)), http.StatusOK
	}

	// Get indexer stats
	stats, err := s.GetIndexerStats(ctx, url, apiKey)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to get indexer stats: %v", err)), http.StatusOK
	}

	// Create health response with version and stats
	health := s.CreateHealthResponse(startTime, "online", "Prowlarr is running")
	health.Version = version
	health.Stats = map[string]interface{}{
		"prowlarr": stats,
	}

	return health, http.StatusOK
}
