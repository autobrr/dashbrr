// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package radarr

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

type RadarrService struct {
	core.ServiceCore
}

func init() {
	models.NewRadarrService = NewRadarrService
}

func NewRadarrService() models.ServiceHealthChecker {
	service := &RadarrService{}
	service.Type = "radarr"
	service.DisplayName = "Radarr"
	service.Description = "Monitor and manage your Radarr instance"
	service.DefaultURL = "http://localhost:7878"
	service.HealthEndpoint = "/api/v3/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *RadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *RadarrService) DeleteQueueItem(ctx context.Context, baseURL, apiKey string, queueId string, options types.RadarrQueueDeleteOptions) error {
	return arr.DeleteQueueItem(ctx, "radarr", baseURL, apiKey, queueId, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

// GetQueue fetches the current queue from Radarr
func (s *RadarrService) GetQueue(ctx context.Context, url, apiKey string) (interface{}, error) {
	if url == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("API key is required")}
	}

	// Build queue URL with query parameters
	queueURL := fmt.Sprintf("%s/api/v3/queue?page=1&pageSize=10&includeUnknownMovieItems=false&includeMovie=false",
		strings.TrimRight(url, "/"))

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, queueURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var queue types.RadarrQueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return queue.Records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []types.RadarrQueueRecord
func (s *RadarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.RadarrQueueRecord, error) {
	records, err := s.GetQueue(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return nil, nil
	}
	return records.([]types.RadarrQueueRecord), nil
}

// LookupByTmdbId fetches movie details from Radarr by TMDB ID
func (s *RadarrService) LookupByTmdbId(ctx context.Context, baseURL, apiKey string, tmdbId int) (*types.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("API key is required")}
	}

	lookupURL := fmt.Sprintf("%s/api/v3/movie/lookup/tmdb?tmdbId=%d", strings.TrimRight(baseURL, "/"), tmdbId)

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, lookupURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie types.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetMovie fetches movie details from Radarr by ID
func (s *RadarrService) GetMovie(ctx context.Context, baseURL, apiKey string, movieID int) (*types.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("API key is required")}
	}

	movieURL := fmt.Sprintf("%s/api/v3/movie/%d", strings.TrimRight(baseURL, "/"), movieID)

	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, movieURL, apiKey, nil)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie types.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &arr.ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetSystemStatus fetches the system status from Radarr
func (s *RadarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatus(ctx, "radarr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Radarr
func (s *RadarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "radarr", url, apiKey)
}

func (s *RadarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
