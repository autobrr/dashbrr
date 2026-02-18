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
	if apiKey == "" {
		return &ErrOverseerr{Message: "Configuration error", Errors: []string{"API key is required"}}
	}

	baseURL := strings.TrimRight(url, "/")
	status := "approve"
	if !approve {
		status = "decline"
	}
	endpoint := fmt.Sprintf("%s/api/v1/request/%d/%s", baseURL, requestID, status)

	headers := map[string]string{
		"X-Api-Key":     apiKey,
		"Content-Type":  "application/json",
		"Cache-Control": "no-cache",
		"Pragma":        "no-cache",
		"Accept":        "application/json",
	}

	resp, err := s.DoRequest(ctx, http.MethodPost, endpoint, headers, []byte("{}"))
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

func (s *OverseerrService) GetRequests(ctx context.Context, url, apiKey string) (*types.RequestsStats, error) {
	if url == "" {
		return nil, &ErrOverseerr{Message: "Configuration error", Errors: []string{"URL is required"}}
	}

	baseURL := strings.TrimRight(url, "/")
	requestEndpoint := fmt.Sprintf("%s/api/v1/request?take=10", baseURL)

	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, requestEndpoint, headers, nil)
	if err != nil {
		return nil, &ErrOverseerr{Message: "Connection error", Errors: []string{err.Error()}}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrOverseerr{Message: "Service error", Errors: []string{err.Error()}}
	}

	var requestsResponse types.RequestsResponse
	if err := json.Unmarshal(body, &requestsResponse); err != nil {
		return nil, &ErrOverseerr{Message: "Response error", Errors: []string{"Failed to parse requests response"}}
	}

	mediaRequests := make([]types.MediaRequest, 0, len(requestsResponse.Results))
	pendingCount := 0

	for _, mediaRequest := range requestsResponse.Results {
		if mediaRequest.Status == 1 { // Pending status
			pendingCount++
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
		"X-Api-Key": apiKey,
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, healthEndpoint, headers, nil)
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
	if err := s.CacheVersion(ctx, url, statusResponse.Version, time.Hour); err != nil {
		log.Warn().
			Err(err).
			Str("url", url).
			Str("version", statusResponse.Version).
			Msg("Failed to cache Overseerr version")
	}

	return s.CreateHealthResponse(startTime, status, message, extras), http.StatusOK
}
