// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sonarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type SonarrService struct {
	core.ServiceCore
}

func init() {
	models.NewSonarrService = NewSonarrService
}

func NewSonarrService() models.ServiceHealthChecker {
	service := &SonarrService{}
	service.Type = "sonarr"
	service.DisplayName = "Sonarr"
	service.Description = "Monitor and manage your Sonarr instance"
	service.DefaultURL = "http://localhost:8989"
	service.HealthEndpoint = "/api/v3/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *SonarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *SonarrService) DeleteQueueItem(ctx context.Context, baseURL, apiKey string, queueId string, options types.SonarrQueueDeleteOptions) error {
	return arr.DeleteQueueItem(ctx, "sonarr", baseURL, apiKey, queueId, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *SonarrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.QueueRecord, error) {
	records, err := arr.FetchQueueRecords[types.QueueRecord](
		ctx,
		"sonarr",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownSeriesItems=false&includeSeries=true&includeEpisode=true",
		s.ReadBody,
	)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetQueue fetches the current queue from Sonarr.
func (s *SonarrService) GetQueue(ctx context.Context, url, apiKey string) (interface{}, error) {
	records, err := s.getQueueRecords(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []types.QueueRecord
func (s *SonarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.QueueRecord, error) {
	return s.getQueueRecords(ctx, url, apiKey)
}

// LookupByTvdbId fetches series details from Sonarr by TVDB ID
func (s *SonarrService) LookupByTvdbId(ctx context.Context, baseURL, apiKey string, tvdbId int) (*types.Series, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("API key is required")}
	}

	lookupURL := fmt.Sprintf("%s/api/v3/series/lookup?term=tvdb%%3A%d", strings.TrimRight(baseURL, "/"), tvdbId)

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, lookupURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var series []types.Series
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Return the first match
	if len(series) > 0 {
		return &series[0], nil
	}

	return nil, &arr.ErrArr{Service: "sonarr", Op: "lookup_tvdb", Err: fmt.Errorf("no series found for TVDB ID: %d", tvdbId)}
}

// GetSeries fetches series details from Sonarr by ID
func (s *SonarrService) GetSeries(ctx context.Context, baseURL, apiKey string, seriesID int) (*types.Series, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", Err: fmt.Errorf("API key is required")}
	}

	seriesURL := fmt.Sprintf("%s/api/v3/series/%d", strings.TrimRight(baseURL, "/"), seriesID)

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, seriesURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var series types.Series
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, &arr.ErrArr{Service: "sonarr", Op: "get_series", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &series, nil
}

// GetSystemStatus fetches the system status from Sonarr
func (s *SonarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatus(ctx, "sonarr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Sonarr
func (s *SonarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "sonarr", url, apiKey)
}

func (s *SonarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
