// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package prowlarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
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
	return arr.MakeArrRequest(ctx, method, url, apiKey, nil)
}

// GetSystemStatus fetches the system status from Prowlarr
func (s *ProwlarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	if url == "" {
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first, ensuring we don't return "true" as a version
	if version := s.GetVersionFromCache(url); version != "" && version != "true" {
		return version, nil
	}

	ctx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	statusURL := fmt.Sprintf("%s/api/v1/system/status", strings.TrimRight(url, "/"))
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
	if err := s.CacheVersion(url, status.Version, time.Hour); err != nil {
		log.Debug().Err(err).Str("url", url).Str("version", status.Version).Msg("Failed to cache Prowlarr version")
	}

	return status.Version, nil
}

// CheckForUpdates checks if there are any updates available for Prowlarr.
func (s *ProwlarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "prowlarr", url, apiKey)
}

// GetIndexerStats fetches indexer statistics from Prowlarr
func (s *ProwlarrService) GetIndexerStats(ctx context.Context, baseURL, apiKey string) (*types.ProwlarrIndexerStatsResponse, error) {
	if baseURL == "" {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("URL is required")}
	}

	statsURL := fmt.Sprintf("%s/api/v1/indexerstats", strings.TrimRight(baseURL, "/"))

	// Add query parameters for date range
	// TODO MAKE THIS CONFIGURABLE IN THE UI
	query := url.Values{}
	query.Add("startDate", "1") // Last 1 day ago
	query.Add("endDate", "30")  // Up to 30 days ago
	statsURL = statsURL + "?" + query.Encode()

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
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
