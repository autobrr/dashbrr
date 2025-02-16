// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/rs/zerolog/log"
)

type RadarrService struct {
	ServiceCore
}

func NewRadarrService(db *database.DB, cache cache.Store, config *domain.ServiceConfiguration) ServiceHealthChecker {
	service := &RadarrService{}
	service.Type = domain.ServiceTypeRadarr
	service.DisplayName = config.DisplayName
	service.Description = "Monitor and manage your Radarr instance"
	service.DefaultURL = "http://localhost:7878"
	service.HealthEndpoint = "/api/v3/health"
	service.URL = config.URL
	service.ApiKey = config.APIKey
	service.InstanceID = config.InstanceID
	service.SetTimeout(DefaultTimeout)
	service.SetDB(db)
	service.SetCache(cache)
	return service
}

func (s *RadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *RadarrService) DeleteQueueItem(ctx context.Context, baseURL, apiKey string, queueId string, options domain.RadarrQueueDeleteOptions) error {
	if baseURL == "" {
		return &ErrArr{Service: "radarr", Op: "delete_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return &ErrArr{Service: "radarr", Op: "delete_queue", Err: fmt.Errorf("API key is required")}
	}

	// Build delete URL with query parameters
	deleteURL := fmt.Sprintf("%s/api/v3/queue/%s?removeFromClient=%t&blocklist=%t&skipRedownload=%t",
		strings.TrimRight(baseURL, "/"),
		queueId,
		options.RemoveFromClient,
		options.Blocklist,
		options.SkipRedownload)

	// Add changeCategory parameter if needed
	if options.ChangeCategory {
		deleteURL += "&changeCategory=true"
	}

	// Log delete attempt with all parameters
	log.Info().
		Str("url", deleteURL).
		Str("queueId", queueId).
		Bool("removeFromClient", options.RemoveFromClient).
		Bool("blocklist", options.Blocklist).
		Bool("skipRedownload", options.SkipRedownload).
		Bool("changeCategory", options.ChangeCategory).
		Msg("Attempting to delete queue item")

	// Execute DELETE request
	resp, err := MakeArrRequest(ctx, http.MethodDelete, deleteURL, apiKey, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", deleteURL).
			Str("queueId", queueId).
			Msg("Failed to execute delete request")
		return &ErrArr{Service: "radarr", Op: "delete_queue", Err: fmt.Errorf("failed to execute request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := s.ReadBody(resp)
		log.Error().
			Int("statusCode", resp.StatusCode).
			Str("url", deleteURL).
			Str("queueId", queueId).
			Str("response", string(body)).
			Msg("Delete request failed")

		var errorResponse struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Message != "" {
			return &ErrArr{Service: "radarr", Op: "delete_queue", Err: fmt.Errorf(errorResponse.Message), HttpCode: resp.StatusCode}
		}
		return &ErrArr{Service: "radarr", Op: "delete_queue", HttpCode: resp.StatusCode}
	}

	log.Info().
		Str("queueId", queueId).
		Msg("Successfully deleted queue item")

	return nil
}

// GetQueue fetches the current queue from Radarr
func (s *RadarrService) GetQueue(ctx context.Context, url, apiKey string) (interface{}, error) {
	if url == "" {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("API key is required")}
	}

	// Build queue URL with query parameters
	queueURL := fmt.Sprintf("%s/api/v3/queue?page=1&pageSize=10&includeUnknownMovieItems=false&includeMovie=false",
		strings.TrimRight(url, "/"))

	resp, err := MakeArrRequest(ctx, http.MethodGet, queueURL, apiKey, nil)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var queue domain.RadarrQueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return queue.Records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []domain.RadarrQueueRecord
func (s *RadarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]domain.RadarrQueueRecord, error) {
	records, err := s.GetQueue(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return nil, nil
	}
	return records.([]domain.RadarrQueueRecord), nil
}

// LookupByTmdbId fetches movie details from Radarr by TMDB ID
func (s *RadarrService) LookupByTmdbId(ctx context.Context, baseURL, apiKey string, tmdbId int) (*domain.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("API key is required")}
	}

	lookupURL := fmt.Sprintf("%s/api/v3/movie/lookup/tmdb?tmdbId=%d", strings.TrimRight(baseURL, "/"), tmdbId)

	resp, err := MakeArrRequest(ctx, http.MethodGet, lookupURL, apiKey, nil)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie domain.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "lookup_tmdb", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetMovie fetches movie details from Radarr by ID
func (s *RadarrService) GetMovie(ctx context.Context, baseURL, apiKey string, movieID int) (*domain.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("API key is required")}
	}

	movieURL := fmt.Sprintf("%s/api/v3/movie/%d", strings.TrimRight(baseURL, "/"), movieID)

	resp, err := MakeArrRequest(ctx, http.MethodGet, movieURL, apiKey, nil)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie domain.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &ErrArr{Service: "radarr", Op: "get_movie", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetSystemStatus fetches the system status from Radarr
func (s *RadarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return GetArrSystemStatus(ctx, "radarr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Radarr
func (s *RadarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return CheckArrForUpdates("radarr", url, apiKey)
}

func (s *RadarrService) CheckHealth(ctx context.Context, url, apiKey string) (*domain.ServiceHealth, int) {
	return ArrHealthCheck(ctx, &s.ServiceCore, s)
}
