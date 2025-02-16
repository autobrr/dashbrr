// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/pkg/errors"
)

// ErrProwlarr Custom error types for better error handling
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
	ServiceCore
}

type ProwlarrSystemStatusResponse struct {
	Version string `json:"version"`
}

func NewProwlarrService(db *database.DB, cache cache.Store, config *domain.ServiceConfiguration) ServiceHealthChecker {
	service := &ProwlarrService{}
	service.Type = domain.ServiceTypeProwlarr
	service.DisplayName = config.DisplayName
	service.Description = "Monitor and manage your Prowlarr instance"
	service.DefaultURL = "http://localhost:9696"
	service.HealthEndpoint = "/api/v1/health"
	service.URL = config.URL
	service.ApiKey = config.APIKey
	service.InstanceID = config.InstanceID
	service.SetTimeout(DefaultTimeout)
	service.SetDB(db)
	service.SetCache(cache)
	return service
}

// makeRequest is a helper function to make requests with proper headers
func (s *ProwlarrService) makeRequest(ctx context.Context, method, url, apiKey string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", apiKey)
	//req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	return client.Do(req)
}

// GetSystemStatus fetches the system status from Prowlarr
// func (s *ProwlarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (*domain.ProwlarrSystemStatusResponse, error) {
func (s *ProwlarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	//if url == "" {
	//	return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	//}

	// Check cache first, ensuring we don't return "true" as a version
	if version := s.GetVersionFromCache(url); version != "" && version != "true" {
		//return nil, nil
		return "", nil
	}

	statusURL := fmt.Sprintf("%s/api/v1/system/status", strings.TrimRight(url, "/"))
	resp, err := s.makeRequest(ctx, http.MethodGet, statusURL, apiKey)
	if err != nil {
		//return nil, &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		//return nil, &ErrProwlarr{Op: "get_system_status", HttpCode: resp.StatusCode}
		return "", &ErrProwlarr{Op: "get_system_status", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		//return nil, &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to read response: %w", err)}
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var data domain.ProwlarrSystemStatusResponse
	if err := json.Unmarshal(body, &data); err != nil {
		//return nil, &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
		return "", &ErrProwlarr{Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// TODO cache full systemstatus

	// Cache version for 1 hour
	if err := s.CacheVersion(nil, url, data.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return data.Version, nil
}

// GetIndexerStats fetches indexer statistics from Prowlarr
func (s *ProwlarrService) GetIndexerStats(ctx context.Context) (*domain.ProwlarrIndexerStatsResponse, error) {
	if s.URL == "" {
		return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("URL is required")}
	}

	cacheKey := domain.CacheKeyProwlarrIndexerStatsPrefix + s.InstanceID

	var data domain.ProwlarrIndexerStatsResponse

	// get from cache
	err := s.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		return &data, nil
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		statsURL := fmt.Sprintf("%s/api/v1/indexerstats", strings.TrimRight(s.URL, "/"))

		// Add query parameters for date range
		// TODO MAKE THIS CONFIGURABLE IN THE UI
		//query := url.Values{}
		//query.Add("startDate", "1") // Last 1 day ago
		//query.Add("endDate", "30")  // Up to 30 days ago
		//statsURL = statsURL + "?" + query.Encode()

		resp, err := s.makeRequest(ctx, http.MethodGet, statsURL, s.ApiKey)
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

		var stats domain.ProwlarrIndexerStatsResponse
		if err := json.Unmarshal(body, &stats); err != nil {
			return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("failed to parse response: %w", err)}
		}

		if err := s.cache.Set(ctx, cacheKey, stats, 5*time.Minute); err != nil {
			return nil, &ErrProwlarr{Op: "get_indexer_stats", Err: fmt.Errorf("failed to cache response: %w", err)}
		}

		return &stats, nil
	}

	return nil, err
}

// GetIndexers fetches indexers from Prowlarr
func (s *ProwlarrService) GetIndexers(ctx context.Context) ([]domain.ProwlarrIndexer, error) {
	if s.URL == "" {
		return nil, &ErrProwlarr{Op: "get_indexers", Err: fmt.Errorf("URL is required")}
	}

	cacheKey := domain.CacheKeyProwlarrIndexerPrefix + s.InstanceID

	var data []domain.ProwlarrIndexer

	// get from cache
	err := s.cache.Get(ctx, cacheKey, &data)
	if err == nil {
		return data, nil
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		statsURL := fmt.Sprintf("%s/api/v1/indexer", strings.TrimRight(s.URL, "/"))

		resp, err := s.makeRequest(ctx, http.MethodGet, statsURL, s.ApiKey)
		if err != nil {
			return nil, &ErrProwlarr{Op: "get_indexers", Err: fmt.Errorf("failed to make request: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, &ErrProwlarr{Op: "get_indexers", HttpCode: resp.StatusCode}
		}

		body, err := s.ReadBody(resp)
		if err != nil {
			return nil, &ErrProwlarr{Op: "get_indexers", Err: fmt.Errorf("failed to read response: %w", err)}
		}

		//data := []domain.ProwlarrIndexer{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, &ErrProwlarr{Op: "get_indexers", Err: fmt.Errorf("failed to parse response: %w", err)}
		}

		if err := s.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
			return nil, &ErrProwlarr{Op: "get_indexers", Err: fmt.Errorf("failed to cache response: %w", err)}
		}

		return data, nil
	}

	return nil, err
}

// CheckForUpdates checks if there are any updates available
func (s *ProwlarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return CheckArrForUpdates("prowlarr", url, apiKey)
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

func (s *ProwlarrService) CheckHealth(ctx context.Context, url, apiKey string) (*domain.ServiceHealth, int) {
	return ArrHealthCheck(ctx, &s.ServiceCore, s)
}
