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

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	// Dashboard default window for indexer stats.
	prowlarrIndexerStatsWindow = 30 * 24 * time.Hour
)

type ProwlarrService struct {
	core.ServiceCore
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

// GetSystemStatus fetches the system status from Prowlarr
func (s *ProwlarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatusWithVersion(ctx, "prowlarr", "v1", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Prowlarr.
func (s *ProwlarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdatesWithVersion(ctx, "prowlarr", "v1", url, apiKey)
}

// GetIndexers fetches indexer configuration and stats baseline from Prowlarr.
func (s *ProwlarrService) GetIndexers(ctx context.Context, baseURL, apiKey string) ([]types.ProwlarrIndexer, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexers", Err: fmt.Errorf("URL is required")}
	}

	indexersURL := fmt.Sprintf("%s/api/v1/indexer", strings.TrimRight(baseURL, "/"))
	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, indexersURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{
			Service: "prowlarr",
			Op:      "get_indexers",
			Err:     fmt.Errorf("failed to make request: %w", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexers", HttpCode: resp.StatusCode}
	}

	var indexers []types.ProwlarrIndexer
	if err := json.NewDecoder(resp.Body).Decode(&indexers); err != nil {
		return nil, &arr.ErrArr{
			Service: "prowlarr",
			Op:      "get_indexers",
			Err:     fmt.Errorf("failed to parse response: %w", err),
		}
	}
	if indexers == nil {
		indexers = []types.ProwlarrIndexer{}
	}

	return indexers, nil
}

// GetIndexerStats fetches indexer statistics from Prowlarr
func (s *ProwlarrService) GetIndexerStats(ctx context.Context, baseURL, apiKey string) (*types.ProwlarrIndexerStatsResponse, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexer_stats", Err: fmt.Errorf("URL is required")}
	}

	statsURL := buildIndexerStatsURL(baseURL, time.Now())

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, statsURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexer_stats", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexer_stats", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexer_stats", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var stats types.ProwlarrIndexerStatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, &arr.ErrArr{Service: "prowlarr", Op: "get_indexer_stats", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &stats, nil
}

func buildIndexerStatsURL(baseURL string, now time.Time) string {
	statsURL := fmt.Sprintf("%s/api/v1/indexerstats", strings.TrimRight(baseURL, "/"))

	endDate := now.UTC()
	startDate := endDate.Add(-prowlarrIndexerStatsWindow)

	query := url.Values{}
	query.Set("startDate", startDate.Format(time.RFC3339))
	query.Set("endDate", endDate.Format(time.RFC3339))

	return statsURL + "?" + query.Encode()
}

// GetQueue gets the current queue status
func (s *ProwlarrService) GetQueue(_ context.Context, _ string, _ string) (any, error) {
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
