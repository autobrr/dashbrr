// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package maintainerr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// Custom error types for better error handling
type ErrMaintainerr struct {
	Op       string // Operation that failed
	Err      error  // Underlying error
	HttpCode int    // HTTP status code if applicable
}

var (
	ErrURLRequired    = errors.New("URL is required")
	ErrAPIKeyRequired = errors.New("API key is required")
)

func (e *ErrMaintainerr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("maintainerr %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("maintainerr %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("maintainerr %s", e.Op)
}

func (e *ErrMaintainerr) Unwrap() error {
	return e.Err
}

type MaintainerrService struct {
	core.ServiceCore
}

type StatusResponse struct {
	Version         string `json:"version"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// Collection declares only the fields that dashbrr consumes. Maintainerr
// changed the types of fields we never read (libraryId, type), and decoding
// them broke parsing entirely (#98).
type Collection struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	IsActive        bool   `json:"isActive"`
	DeleteAfterDays int    `json:"deleteAfterDays"`
	// MediaCount is the total media count of the collection. Newer Maintainerr
	// returns it directly and caps media at a small preview. Older versions
	// omit it, so GetCollections falls back to len(Media).
	MediaCount int               `json:"mediaCount"`
	Media      []json.RawMessage `json:"media"`
}

func init() {
	models.NewMaintainerrService = NewMaintainerrService
}

func NewMaintainerrService() models.ServiceHealthChecker {
	service := &MaintainerrService{}
	service.Type = "maintainerr"
	service.DisplayName = "Maintainerr"
	service.Description = "Monitor and manage your Maintainerr instance"
	service.DefaultURL = "http://localhost:6246"
	service.HealthEndpoint = "/api/app/status"
	service.SetTimeout(core.DefaultLongTimeout)
	return service
}

func (s *MaintainerrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/app/status", baseURL)
}

func (s *MaintainerrService) getVersion(ctx context.Context, url string) (string, error) {
	// Check cache first, ensuring we don't return "true" as a version
	if version := s.GetVersionFromCache(ctx, url); version != "" && version != "true" {
		return version, nil
	}

	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.DoRequest(ctx, http.MethodGet, healthEndpoint, nil, nil)
	if err != nil {
		return "", &ErrMaintainerr{Op: "get_version", Err: fmt.Errorf("failed to make request: %w", err)}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrMaintainerr{Op: "get_version", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return "", &ErrMaintainerr{Op: "get_version", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(ctx, url, statusResponse.Version, time.Hour); err != nil {
		log.Debug().Err(err).Str("url", url).Str("version", statusResponse.Version).Msg("Failed to cache Maintainerr version")
	}

	if err := s.CacheUpdateStatus(ctx, url, statusResponse.UpdateAvailable, time.Hour); err != nil {
		log.Debug().Err(err).Str("url", url).Bool("updateAvailable", statusResponse.UpdateAvailable).Msg("Failed to cache Maintainerr update status")
	}

	return statusResponse.Version, nil
}

func (s *MaintainerrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	// Create a child context with longer timeout if needed
	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultLongTimeout)
	defer cancel()

	versionChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		version, err := s.getVersion(healthCtx, url)
		if err != nil {
			errChan <- err
			versionChan <- ""
			return
		}
		versionChan <- version
		errChan <- nil
	}()

	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.DoRequest(healthCtx, http.MethodGet, healthEndpoint, nil, nil)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	// Calculate response time directly
	responseTime := time.Since(startTime).Milliseconds()

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

	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to parse status response: %v", err)), http.StatusOK
	}

	var version string
	var versionErr error
	select {
	case version = <-versionChan:
		versionErr = <-errChan
	case <-healthCtx.Done():
		versionErr = healthCtx.Err()
	}

	if err := s.CacheUpdateStatus(ctx, url, statusResponse.UpdateAvailable, time.Hour); err != nil {
		log.Debug().Err(err).Str("url", url).Bool("updateAvailable", statusResponse.UpdateAvailable).Msg("Failed to cache Maintainerr update status")
	}

	extras := map[string]any{
		"updateAvailable": statusResponse.UpdateAvailable,
		"responseTime":    responseTime,
	}

	if version != "" {
		extras["version"] = version
	}
	if versionErr != nil {
		extras["versionError"] = versionErr.Error()
	}

	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}

func (s *MaintainerrService) GetCollections(ctx context.Context, url, apiKey string) ([]Collection, error) {
	if url == "" {
		return nil, &ErrMaintainerr{Op: "get_collections", Err: ErrURLRequired}
	}

	if apiKey == "" {
		return nil, &ErrMaintainerr{Op: "get_collections", Err: ErrAPIKeyRequired}
	}

	baseURL := strings.TrimRight(url, "/")
	endpoint := fmt.Sprintf("%s/api/collections", baseURL)

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, &ErrMaintainerr{Op: "get_collections", Err: fmt.Errorf("failed to connect: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrMaintainerr{
			Op:       "get_collections",
			HttpCode: resp.StatusCode,
		}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, &ErrMaintainerr{Op: "get_collections", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	var collections []Collection
	if err := json.Unmarshal(body, &collections); err != nil {
		return nil, &ErrMaintainerr{Op: "get_collections", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	activeCollections := make([]Collection, 0)
	for _, collection := range collections {
		if collection.IsActive {
			if collection.MediaCount == 0 {
				collection.MediaCount = len(collection.Media)
			}
			activeCollections = append(activeCollections, collection)
		}
	}

	return activeCollections, nil
}
