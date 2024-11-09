// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package radarr

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
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

// Custom error types for better error handling
type ErrRadarr struct {
	Op       string // Operation that failed
	Err      error  // Underlying error
	HttpCode int    // HTTP status code if applicable
}

func (e *ErrRadarr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("radarr %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("radarr %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("radarr %s", e.Op)
}

func (e *ErrRadarr) Unwrap() error {
	return e.Err
}

type RadarrService struct {
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
	models.NewRadarrService = NewRadarrService
}

func NewRadarrService() models.ServiceHealthChecker {
	service := &RadarrService{}
	service.Type = "radarr"
	service.DisplayName = "Radarr"
	service.Description = "Monitor and manage your Radarr instance"
	service.DefaultURL = "http://localhost:7878"
	service.HealthEndpoint = "/api/v3/health"
	return service
}

func (s *RadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// makeRequest is a helper function to make requests with proper headers
func (s *RadarrService) makeRequest(ctx context.Context, method, url, apiKey string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set headers correctly
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	// Log the request details
	log.Debug().
		Str("method", method).
		Str("url", url).
		Msg("Making request to Radarr API")

	client := &http.Client{}
	return client.Do(req)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *RadarrService) DeleteQueueItem(baseURL, apiKey string, queueId string, options types.RadarrQueueDeleteOptions) error {
	if baseURL == "" {
		return &ErrRadarr{Op: "delete_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return &ErrRadarr{Op: "delete_queue", Err: fmt.Errorf("API key is required")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		return &ErrRadarr{Op: "delete_queue", Err: fmt.Errorf("failed to execute request: %w", err)}
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
			return &ErrRadarr{Op: "delete_queue", Err: fmt.Errorf(errorResponse.Message), HttpCode: resp.StatusCode}
		}
		return &ErrRadarr{Op: "delete_queue", HttpCode: resp.StatusCode}
	}

	log.Info().
		Str("queueId", queueId).
		Msg("Successfully deleted queue item")

	return nil
}

// GetQueue fetches the current queue from Radarr
func (s *RadarrService) GetQueue(url, apiKey string) ([]types.RadarrQueueRecord, error) {
	if url == "" {
		return nil, &ErrRadarr{Op: "get_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrRadarr{Op: "get_queue", Err: fmt.Errorf("API key is required")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build queue URL with query parameters
	queueURL := fmt.Sprintf("%s/api/v3/queue?page=1&pageSize=10&includeUnknownMovieItems=false&includeMovie=false",
		strings.TrimRight(url, "/"))

	log.Debug().
		Str("url", queueURL).
		Msg("Fetching queue")

	resp, err := s.makeRequest(ctx, http.MethodGet, queueURL, apiKey, nil)
	if err != nil {
		return nil, &ErrRadarr{Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrRadarr{Op: "get_queue", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrRadarr{Op: "get_queue", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var queue types.RadarrQueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, &ErrRadarr{Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return queue.Records, nil
}

// LookupByTmdbId fetches movie details from Radarr by TMDB ID
func (s *RadarrService) LookupByTmdbId(baseURL, apiKey string, tmdbId int) (*types.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &ErrRadarr{Op: "lookup_tmdb", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrRadarr{Op: "lookup_tmdb", Err: fmt.Errorf("API key is required")}
	}

	lookupURL := fmt.Sprintf("%s/api/v3/movie/lookup/tmdb?tmdbId=%d", strings.TrimRight(baseURL, "/"), tmdbId)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, lookupURL, apiKey, nil)
	if err != nil {
		return nil, &ErrRadarr{Op: "lookup_tmdb", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrRadarr{Op: "lookup_tmdb", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrRadarr{Op: "lookup_tmdb", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie types.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &ErrRadarr{Op: "lookup_tmdb", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetMovie fetches movie details from Radarr by ID
func (s *RadarrService) GetMovie(baseURL, apiKey string, movieID int) (*types.RadarrMovieResponse, error) {
	if baseURL == "" {
		return nil, &ErrRadarr{Op: "get_movie", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrRadarr{Op: "get_movie", Err: fmt.Errorf("API key is required")}
	}

	movieURL := fmt.Sprintf("%s/api/v3/movie/%d", strings.TrimRight(baseURL, "/"), movieID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, movieURL, apiKey, nil)
	if err != nil {
		return nil, &ErrRadarr{Op: "get_movie", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrRadarr{Op: "get_movie", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrRadarr{Op: "get_movie", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var movie types.RadarrMovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, &ErrRadarr{Op: "get_movie", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return &movie, nil
}

// GetSystemStatus fetches the system status from Radarr
func (s *RadarrService) GetSystemStatus(baseURL, apiKey string) (string, error) {
	if baseURL == "" {
		return "", &ErrRadarr{Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first
	if version := s.GetVersionFromCache(baseURL); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v3/system/status", strings.TrimRight(baseURL, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, statusURL, apiKey, nil)
	if err != nil {
		return "", &ErrRadarr{Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrRadarr{Op: "get_system_status", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrRadarr{Op: "get_system_status", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var status SystemStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return "", &ErrRadarr{Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(baseURL, status.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return status.Version, nil
}

// CheckForUpdates checks if there are any updates available for Radarr
func (s *RadarrService) CheckForUpdates(baseURL, apiKey string) (bool, error) {
	if baseURL == "" {
		return false, &ErrRadarr{Op: "check_for_updates", Err: fmt.Errorf("URL is required")}
	}

	updateURL := fmt.Sprintf("%s/api/v3/update", strings.TrimRight(baseURL, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.makeRequest(ctx, http.MethodGet, updateURL, apiKey, nil)
	if err != nil {
		return false, &ErrRadarr{Op: "check_for_updates", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, &ErrRadarr{Op: "check_for_updates", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return false, &ErrRadarr{Op: "check_for_updates", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var updates []types.UpdateResponse
	if err := json.Unmarshal(body, &updates); err != nil {
		return false, &ErrRadarr{Op: "check_for_updates", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Check if there's any update available
	for _, update := range updates {
		if !update.Installed && update.Installable {
			return true, nil
		}
	}

	return false, nil
}

func (s *RadarrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
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

	// Start update check in background
	updateChan := make(chan bool, 1)
	updateErrChan := make(chan error, 1)
	go func() {
		hasUpdate, err := s.CheckForUpdates(url, apiKey)
		updateChan <- hasUpdate
		updateErrChan <- err
	}()

	// Start queue check in background
	queueChan := make(chan []types.RadarrQueueRecord, 1)
	queueErrChan := make(chan error, 1)
	go func() {
		queue, err := s.GetQueue(url, apiKey)
		queueChan <- queue
		queueErrChan <- err
	}()

	// Perform health check
	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.makeRequest(ctx, http.MethodGet, healthEndpoint, apiKey, nil)
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

	// Wait for update check with timeout
	var updateAvailable bool
	var updateErr error
	select {
	case updateAvailable = <-updateChan:
		updateErr = <-updateErrChan
	case <-time.After(500 * time.Millisecond):
		// Continue without update check if it takes too long
	}

	// Wait for queue with timeout
	var queue []types.RadarrQueueRecord
	var queueErr error
	select {
	case queue = <-queueChan:
		queueErr = <-queueErrChan
	case <-time.After(500 * time.Millisecond):
		// Continue without queue if it takes too long
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

	if updateAvailable {
		extras["updateAvailable"] = true
	}
	if updateErr != nil {
		extras["updateError"] = updateErr.Error()
	}

	if queueErr != nil {
		extras["queueError"] = queueErr.Error()
	}

	var allWarnings []string

	// Enhanced health issues check
	for _, issue := range healthIssues {
		if issue.Type == "warning" || issue.Type == "error" {
			var warning string

			// Handle notifications and indexers without source prefix and wiki
			if strings.Contains(issue.Message, "Notifications unavailable") ||
				strings.Contains(issue.Message, "Indexers unavailable") {
				warning = issue.Message
			} else {
				// For other types of warnings, include source and wiki
				warning = issue.Message
				if issue.WikiURL != "" {
					warning += fmt.Sprintf("\nWiki: %s", issue.WikiURL)
				}
				if issue.Source != "" &&
					issue.Source != "IndexerLongTermStatusCheck" &&
					issue.Source != "NotificationStatusCheck" {
					warning = fmt.Sprintf("[%s] %s", issue.Source, warning)
				}
			}

			allWarnings = append(allWarnings, warning)
		}
	}

	// Check queue for warning status
	if queue != nil {
		for _, record := range queue {
			if record.TrackedDownloadStatus == "warning" {
				warning := fmt.Sprintf("%s:", record.Title)
				for _, msg := range record.StatusMessages {
					for _, message := range msg.Messages {
						warning += "\n" + message
					}
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
