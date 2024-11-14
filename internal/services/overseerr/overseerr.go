// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package overseerr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/types"
)

// ErrOverseerr is a custom error type for Overseerr-specific errors
type ErrOverseerr struct {
	Message string
	Errors  []string
}

func (e *ErrOverseerr) Error() string {
	if len(e.Errors) == 0 {
		return e.Message
	}

	// Format message with bullet points for each error
	errorList := strings.Join(e.Errors, "\n• ")
	return fmt.Sprintf("%s:\n• %s", e.Message, errorList)
}

type OverseerrService struct {
	core.ServiceCore
	db *database.DB
}

func init() {
	models.NewOverseerrService = NewOverseerrService
}

func NewOverseerrService() models.ServiceHealthChecker {
	service := &OverseerrService{}
	service.Type = "overseerr"
	service.DisplayName = "Overseerr"
	service.Description = "Monitor and manage your Overseerr instance"
	service.DefaultURL = "http://localhost:5055"
	service.HealthEndpoint = "/api/v1/status"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *OverseerrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/status", baseURL)
}

// SetDB sets the database instance for the service
func (s *OverseerrService) SetDB(db *database.DB) {
	s.db = db
}

// UpdateRequestStatus updates the status of a media request (approve/reject)
func (s *OverseerrService) UpdateRequestStatus(ctx context.Context, url, apiKey string, requestID int, approve bool) error {
	if url == "" {
		return &ErrOverseerr{Message: "Configuration error", Errors: []string{"URL is required"}}
	}

	baseURL := strings.TrimRight(url, "/")
	status := "approve"
	if !approve {
		status = "decline"
	}
	endpoint := fmt.Sprintf("%s/api/v1/request/%d/%s", baseURL, requestID, status)

	// Create an empty request body
	emptyBody := bytes.NewReader([]byte("{}"))

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, emptyBody)
	if err != nil {
		return &ErrOverseerr{Message: "Failed to create request", Errors: []string{err.Error()}}
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &ErrOverseerr{Message: "Connection error", Errors: []string{err.Error()}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrOverseerr{
			Message: "Failed to update request status",
			Errors:  []string{fmt.Sprintf("Server returned status code: %d", resp.StatusCode)},
		}
	}

	return nil
}

// fetchMediaTitle fetches the title from either Radarr or Sonarr based on mediaType
func (s *OverseerrService) fetchMediaTitle(ctx context.Context, request types.MediaRequest) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	var service *models.ServiceConfiguration
	var err error

	switch request.Media.MediaType {
	case "movie":
		// Find Radarr service by URL
		service, err = s.db.GetServiceByInstancePrefix("radarr")
		if err != nil {
			return "", fmt.Errorf("failed to get Radarr service: %w", err)
		}
		if service == nil {
			return "", fmt.Errorf("no Radarr service found")
		}

		radarrService := &radarr.RadarrService{}
		// Use TmdbID for movie lookups
		movie, err := radarrService.LookupByTmdbId(ctx, service.URL, service.APIKey, request.Media.TmdbID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch movie from Radarr: %w", err)
		}
		return movie.Title, nil

	case "tv":
		// Find Sonarr service by URL
		service, err = s.db.GetServiceByInstancePrefix("sonarr")
		if err != nil {
			return "", fmt.Errorf("failed to get Sonarr service: %w", err)
		}
		if service == nil {
			return "", fmt.Errorf("no Sonarr service found")
		}

		sonarrService := &sonarr.SonarrService{}
		// Use TvdbID for TV show lookups
		series, err := sonarrService.LookupByTvdbId(ctx, service.URL, service.APIKey, request.Media.TvdbID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch series from Sonarr: %w", err)
		}
		return series.Title, nil

	default:
		return "", fmt.Errorf("unknown media type: %s", request.Media.MediaType)
	}
}

func (s *OverseerrService) GetRequests(ctx context.Context, url, apiKey string) (*types.RequestsStats, error) {
	if url == "" {
		return nil, &ErrOverseerr{Message: "Configuration error", Errors: []string{"URL is required"}}
	}

	baseURL := strings.TrimRight(url, "/")
	requestEndpoint := fmt.Sprintf("%s/api/v1/request?take=10", baseURL)

	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, requestEndpoint, "", headers)
	if err != nil {
		return nil, &ErrOverseerr{Message: "Connection error", Errors: []string{err.Error()}}
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrOverseerr{Message: "Service error", Errors: []string{err.Error()}}
	}

	var requestsResponse types.RequestsResponse
	if err := json.Unmarshal(body, &requestsResponse); err != nil {
		return nil, &ErrOverseerr{Message: "Response error", Errors: []string{"Failed to parse requests response"}}
	}

	// Convert the generic results to MediaRequest structs and count pending
	mediaRequests := make([]types.MediaRequest, 0)
	pendingCount := 0

	for _, result := range requestsResponse.Results {
		resultBytes, err := json.Marshal(result)
		if err != nil {
			continue
		}

		var mediaRequest types.MediaRequest
		if err := json.Unmarshal(resultBytes, &mediaRequest); err != nil {
			continue
		}

		if mediaRequest.Status == 1 { // Pending status
			pendingCount++
		}

		// Try to fetch the title using the appropriate lookup method
		title, err := s.fetchMediaTitle(ctx, mediaRequest)
		if err == nil {
			mediaRequest.Media.Title = title
		}

		mediaRequests = append(mediaRequests, mediaRequest)
	}

	return &types.RequestsStats{
		PendingCount: pendingCount,
		Requests:     mediaRequests,
	}, nil
}

func (s *OverseerrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", (&ErrOverseerr{
			Message: "Configuration error",
			Errors:  []string{"URL is required"},
		}).Error()), http.StatusBadRequest
	}

	healthEndpoint := s.GetHealthEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", (&ErrOverseerr{
			Message: "Connection error",
			Errors:  []string{err.Error()},
		}).Error()), http.StatusOK
	}
	defer resp.Body.Close()

	// Calculate response time directly
	responseTime := time.Since(startTime).Milliseconds()

	body, err := s.ReadBody(resp)
	if err != nil {
		errMsg := (&ErrOverseerr{
			Message: "Service error",
			Errors:  []string{err.Error()},
		}).Error()

		// Align error status with request failures
		if resp.StatusCode >= 500 {
			return s.CreateHealthResponse(startTime, "error", errMsg), http.StatusOK
		}
		return s.CreateHealthResponse(startTime, "warning", errMsg), http.StatusOK
	}

	// Parse the response
	var statusResponse types.StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return s.CreateHealthResponse(startTime, "warning", (&ErrOverseerr{
			Message: "Response error",
			Errors:  []string{"Failed to parse status response"},
		}).Error()), http.StatusOK
	}

	// Create response with version, update information, and response time
	extras := map[string]interface{}{
		"version":         statusResponse.Version,
		"updateAvailable": statusResponse.UpdateAvailable,
		"responseTime":    responseTime,
	}

	status := "online"
	message := "healthy"

	if statusResponse.Status != 0 {
		if statusResponse.Status >= 400 {
			status = "warning"
			message = (&ErrOverseerr{
				Message: "Service warning",
				Errors:  []string{fmt.Sprintf("Service reported status code: %d", statusResponse.Status)},
			}).Error()
		}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(url, statusResponse.Version, time.Hour); err != nil {
		log.Warn().
			Err(err).
			Str("url", url).
			Str("version", statusResponse.Version).
			Msg("Failed to cache Overseerr version")
	}

	return s.CreateHealthResponse(startTime, status, message, extras), http.StatusOK
}
