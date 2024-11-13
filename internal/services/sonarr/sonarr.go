// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

// Custom error types for better error handling
type ErrSonarr struct {
	Op       string // Operation that failed
	Err      error  // Underlying error
	HttpCode int    // HTTP status code if applicable
}

func (e *ErrSonarr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("sonarr %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("sonarr %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("sonarr %s", e.Op)
}

func (e *ErrSonarr) Unwrap() error {
	return e.Err
}

type SonarrService struct {
	core.ServiceCore
}

type SystemStatusResponse struct {
	Version string `json:"version"`
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

// makeRequest is a helper function to make requests with proper headers
func (s *SonarrService) makeRequest(ctx context.Context, method, url, apiKey string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set headers correctly
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	return client.Do(req)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *SonarrService) DeleteQueueItem(baseURL, apiKey string, queueId string, options types.SonarrQueueDeleteOptions) error {
	if baseURL == "" {
		return &ErrSonarr{Op: "delete_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return &ErrSonarr{Op: "delete_queue", Err: fmt.Errorf("API key is required")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

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
	resp, err := s.makeRequest(ctx, http.MethodDelete, deleteURL, apiKey, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", deleteURL).
			Str("queueId", queueId).
			Msg("Failed to execute delete request")
		return &ErrSonarr{Op: "delete_queue", Err: fmt.Errorf("failed to execute request: %w", err)}
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
			return &ErrSonarr{Op: "delete_queue", Err: fmt.Errorf(errorResponse.Message), HttpCode: resp.StatusCode}
		}
		return &ErrSonarr{Op: "delete_queue", HttpCode: resp.StatusCode}
	}

	log.Info().
		Str("queueId", queueId).
		Msg("Successfully deleted queue item")

	return nil
}

// GetQueue fetches the current queue from Sonarr
func (s *SonarrService) GetQueue(url, apiKey string) (interface{}, error) {
	if url == "" {
		return nil, &ErrSonarr{Op: "get_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrSonarr{Op: "get_queue", Err: fmt.Errorf("API key is required")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	queueURL := fmt.Sprintf("%s/api/v3/queue?page=1&pageSize=10&includeUnknownSeriesItems=false&includeSeries=true&includeEpisode=true",
		strings.TrimRight(url, "/"))

	resp, err := s.makeRequest(ctx, http.MethodGet, queueURL, apiKey, nil)
	if err != nil {
		return nil, &ErrSonarr{Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrSonarr{Op: "get_queue", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrSonarr{Op: "get_queue", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var queue types.SonarrQueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, &ErrSonarr{Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return queue.Records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []types.QueueRecord
func (s *SonarrService) GetQueueForHealth(url, apiKey string) ([]types.QueueRecord, error) {
	records, err := s.GetQueue(url, apiKey)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return nil, nil
	}
	return records.([]types.QueueRecord), nil
}

// LookupByTvdbId fetches series details from Sonarr by TVDB ID
func (s *SonarrService) LookupByTvdbId(baseURL, apiKey string, tvdbId int) (*types.Series, error) {
	if baseURL == "" {
		return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("API key is required")}
	}

	lookupURL := fmt.Sprintf("%s/api/v3/series/lookup?term=tvdb%%3A%d", strings.TrimRight(baseURL, "/"), tvdbId)
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, lookupURL, apiKey, nil)
	if err != nil {
		return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrSonarr{Op: "lookup_tvdb", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var series []types.Series
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Return the first match
	if len(series) > 0 {
		return &series[0], nil
	}

	return nil, &ErrSonarr{Op: "lookup_tvdb", Err: fmt.Errorf("no series found for TVDB ID: %d", tvdbId)}
}

// GetSeries fetches series details from Sonarr by ID
func (s *SonarrService) GetSeries(baseURL, apiKey string, seriesID int) (*types.Series, error) {
	if baseURL == "" {
		return nil, &ErrSonarr{Op: "get_series", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrSonarr{Op: "get_series", Err: fmt.Errorf("API key is required")}
	}

	seriesURL := fmt.Sprintf("%s/api/v3/series/%d", strings.TrimRight(baseURL, "/"), seriesID)
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, seriesURL, apiKey, nil)
	if err != nil {
		return nil, &ErrSonarr{Op: "get_series", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrSonarr{Op: "get_series", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrSonarr{Op: "get_series", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var series types.Series
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, &ErrSonarr{Op: "get_series", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &series, nil
}

// GetSystemStatus fetches the system status from Sonarr
func (s *SonarrService) GetSystemStatus(url, apiKey string) (string, error) {
	if url == "" {
		return "", &ErrSonarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first
	if version := s.GetVersionFromCache(url); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v3/system/status", strings.TrimRight(url, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, statusURL, apiKey, nil)
	if err != nil {
		return "", &ErrSonarr{Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrSonarr{Op: "get_system_status", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrSonarr{Op: "get_system_status", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var status SystemStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return "", &ErrSonarr{Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(url, status.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return status.Version, nil
}

// CheckForUpdates checks if there are any updates available for Sonarr
func (s *SonarrService) CheckForUpdates(url, apiKey string) (bool, error) {
	if url == "" {
		return false, &ErrSonarr{Op: "check_for_updates", Err: fmt.Errorf("URL is required")}
	}

	updateURL := fmt.Sprintf("%s/api/v3/update", strings.TrimRight(url, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, updateURL, apiKey, nil)
	if err != nil {
		return false, &ErrSonarr{Op: "check_for_updates", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, &ErrSonarr{Op: "check_for_updates", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return false, &ErrSonarr{Op: "check_for_updates", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var updates []types.SonarrUpdateResponse
	if err := json.Unmarshal(body, &updates); err != nil {
		return false, &ErrSonarr{Op: "check_for_updates", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Check if there's any update available
	for _, update := range updates {
		if !update.Installed && update.Installable {
			return true, nil
		}
	}

	return false, nil
}

func (s *SonarrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(&s.ServiceCore, url, apiKey, s)
}
