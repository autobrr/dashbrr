// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package overseerr

import (
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

type StatusResponse struct {
	Version         string `json:"version"`
	CommitTag       string `json:"commitTag"`
	Status          int    `json:"status"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

type RequestsResponse struct {
	PageInfo struct {
		Pages    int `json:"pages"`
		PageSize int `json:"pageSize"`
		Results  int `json:"results"`
		Page     int `json:"page"`
	} `json:"pageInfo"`
	Results []interface{} `json:"results"`
}

type MediaRequest struct {
	ID        int       `json:"id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Media     struct {
		ID                int      `json:"id"`
		TmdbID            int      `json:"tmdbId"`
		TvdbID            int      `json:"tvdbId"`
		Status            int      `json:"status"`
		Requests          []string `json:"requests"`
		CreatedAt         string   `json:"createdAt"`
		UpdatedAt         string   `json:"updatedAt"`
		MediaType         string   `json:"mediaType"`
		ServiceUrl        string   `json:"serviceUrl"`
		Title             string   `json:"title,omitempty"`
		ExternalServiceID int      `json:"externalServiceId,omitempty"`
	} `json:"media"`
	RequestedBy struct {
		ID           int    `json:"id"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		PlexToken    string `json:"plexToken"`
		PlexUsername string `json:"plexUsername"`
		UserType     int    `json:"userType"`
		Permissions  int    `json:"permissions"`
		Avatar       string `json:"avatar"`
		CreatedAt    string `json:"createdAt"`
		UpdatedAt    string `json:"updatedAt"`
		RequestCount int    `json:"requestCount"`
	} `json:"requestedBy"`
	ModifiedBy struct {
		ID           int    `json:"id"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		PlexToken    string `json:"plexToken"`
		PlexUsername string `json:"plexUsername"`
		UserType     int    `json:"userType"`
		Permissions  int    `json:"permissions"`
		Avatar       string `json:"avatar"`
		CreatedAt    string `json:"createdAt"`
		UpdatedAt    string `json:"updatedAt"`
		RequestCount int    `json:"requestCount"`
	} `json:"modifiedBy"`
	Is4k       bool   `json:"is4k"`
	ServerID   int    `json:"serverId"`
	ProfileID  int    `json:"profileId"`
	RootFolder string `json:"rootFolder"`
}

type RequestsStats struct {
	PendingCount int            `json:"pendingCount"`
	Requests     []MediaRequest `json:"requests"`
}

func init() {
	models.NewOverseerrService = func() models.ServiceHealthChecker {
		return &OverseerrService{}
	}
}

func (s *OverseerrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/status", baseURL)
}

// SetDB sets the database instance for the service
func (s *OverseerrService) SetDB(db *database.DB) {
	s.db = db
}

// fetchMediaTitle fetches the title from either Radarr or Sonarr based on mediaType
func (s *OverseerrService) fetchMediaTitle(request MediaRequest) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	if request.Media.ExternalServiceID == 0 {
		return "", fmt.Errorf("no external service ID provided")
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
		movie, err := radarrService.GetMovie(service.URL, service.APIKey, request.Media.ExternalServiceID)
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
		series, err := sonarrService.GetSeries(service.URL, service.APIKey, request.Media.ExternalServiceID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch series from Sonarr: %w", err)
		}
		return series.Title, nil

	default:
		return "", fmt.Errorf("unknown media type: %s", request.Media.MediaType)
	}
}

func (s *OverseerrService) GetRequests(url, apiKey string) (*RequestsStats, error) {
	if url == "" {
		return nil, &ErrOverseerr{Message: "Configuration error", Errors: []string{"URL is required"}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(url, "/")
	requestEndpoint := fmt.Sprintf("%s/api/v1/request?take=10", baseURL)

	log.Debug().
		Str("url", url).
		Str("endpoint", requestEndpoint).
		Msg("Fetching Overseerr requests")

	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, requestEndpoint, "", headers)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", url).
			Msg("Failed to connect to Overseerr")
		return nil, &ErrOverseerr{Message: "Connection error", Errors: []string{err.Error()}}
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", url).
			Msg("Failed to read Overseerr response body")
		return nil, &ErrOverseerr{Message: "Service error", Errors: []string{err.Error()}}
	}

	var requestsResponse RequestsResponse
	if err := json.Unmarshal(body, &requestsResponse); err != nil {
		log.Error().
			Err(err).
			Str("url", url).
			Str("body", string(body)).
			Msg("Failed to parse Overseerr response")
		return nil, &ErrOverseerr{Message: "Response error", Errors: []string{"Failed to parse requests response"}}
	}

	// Convert the generic results to MediaRequest structs and count pending
	mediaRequests := make([]MediaRequest, 0)
	pendingCount := 0

	for _, result := range requestsResponse.Results {
		resultBytes, err := json.Marshal(result)
		if err != nil {
			log.Warn().
				Err(err).
				Interface("result", result).
				Msg("Failed to marshal request result")
			continue
		}

		var mediaRequest MediaRequest
		if err := json.Unmarshal(resultBytes, &mediaRequest); err != nil {
			log.Warn().
				Err(err).
				Str("resultBytes", string(resultBytes)).
				Msg("Failed to unmarshal media request")
			continue
		}

		if mediaRequest.Status == 1 { // Pending status
			pendingCount++
		}

		// If we have an external service ID, try to fetch the title
		if mediaRequest.Media.ExternalServiceID != 0 {
			title, err := s.fetchMediaTitle(mediaRequest)
			if err != nil {
				log.Warn().
					Err(err).
					Str("mediaType", mediaRequest.Media.MediaType).
					Int("externalServiceId", mediaRequest.Media.ExternalServiceID).
					Msg("Failed to fetch media title")
			} else {
				mediaRequest.Media.Title = title
			}
		}

		mediaRequests = append(mediaRequests, mediaRequest)
	}

	log.Debug().
		Int("totalRequests", len(mediaRequests)).
		Int("pendingCount", pendingCount).
		Msg("Successfully processed Overseerr requests")

	return &RequestsStats{
		PendingCount: pendingCount,
		Requests:     mediaRequests,
	}, nil
}

func (s *OverseerrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", (&ErrOverseerr{
			Message: "Configuration error",
			Errors:  []string{"URL is required"},
		}).Error()), http.StatusBadRequest
	}

	// Create a context with timeout for the health check
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	healthEndpoint := s.GetHealthEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", url).
			Msg("Failed to connect to Overseerr health endpoint")
		return s.CreateHealthResponse(startTime, "offline", (&ErrOverseerr{
			Message: "Connection error",
			Errors:  []string{err.Error()},
		}).Error()), http.StatusOK
	}
	defer resp.Body.Close()

	// Get response time from header
	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		errMsg := (&ErrOverseerr{
			Message: "Service error",
			Errors:  []string{err.Error()},
		}).Error()

		log.Error().
			Err(err).
			Str("url", url).
			Msg("Failed to read Overseerr health response")

		// Align error status with request failures
		if resp.StatusCode >= 500 {
			return s.CreateHealthResponse(startTime, "error", errMsg), http.StatusOK
		}
		return s.CreateHealthResponse(startTime, "warning", errMsg), http.StatusOK
	}

	// Parse the response
	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		log.Error().
			Err(err).
			Str("url", url).
			Str("body", string(body)).
			Msg("Failed to parse Overseerr health response")
		return s.CreateHealthResponse(startTime, "warning", (&ErrOverseerr{
			Message: "Response error",
			Errors:  []string{"Failed to parse status response"},
		}).Error()), http.StatusOK
	}

	// Create response with version, update information, and response time
	extras := map[string]interface{}{
		"version":         statusResponse.Version,
		"updateAvailable": statusResponse.UpdateAvailable,
		"responseTime":    responseTime.Milliseconds(),
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
