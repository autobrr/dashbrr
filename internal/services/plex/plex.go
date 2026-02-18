// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package plex

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type PlexService struct {
	core.ServiceCore
}

func init() {
	models.NewPlexService = NewPlexService
}

func NewPlexService() models.ServiceHealthChecker {
	service := &PlexService{
		ServiceCore: core.ServiceCore{
			Type:           "plex",
			DisplayName:    "Plex",
			Description:    "Monitor and manage your Plex Media Server",
			DefaultURL:     "http://localhost:32400",
			HealthEndpoint: "/identity",
		},
	}
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *PlexService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/identity", baseURL)
}

func (s *PlexService) getPlexHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Accept":                   "application/json",
		"X-Plex-Token":             apiKey,
		"X-Plex-Client-Identifier": "com.dashbrr.app",
		"X-Plex-Product":           "Dashbrr",
		"X-Plex-Version":           "1.0.0",
		"X-Plex-Platform":          "Web",
		"X-Plex-Device":            "Browser",
	}
}

func (s *PlexService) getPlexAuthHeaders(clientIdentifier, product string) map[string]string {
	if strings.TrimSpace(product) == "" {
		product = "Dashbrr"
	}

	return map[string]string{
		"Accept":                   "application/json",
		"X-Plex-Client-Identifier": clientIdentifier,
		"X-Plex-Product":           product,
		"X-Plex-Version":           "1.0.0",
		"X-Plex-Platform":          "Web",
		"X-Plex-Device":            "Browser",
	}
}

func (s *PlexService) CreateAuthPIN(ctx context.Context, clientIdentifier, product string) (*types.PlexPIN, error) {
	if strings.TrimSpace(clientIdentifier) == "" {
		return nil, fmt.Errorf("client identifier is required")
	}

	endpoint := "https://plex.tv/api/v2/pins?strong=true"
	resp, err := s.DoRequest(ctx, http.MethodPost, endpoint, s.getPlexAuthHeaders(clientIdentifier, product), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create plex auth pin: %w", err)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read plex auth pin response: %w", err)
	}

	var pin types.PlexPIN
	if err := json.Unmarshal(body, &pin); err != nil {
		return nil, fmt.Errorf("failed to parse plex auth pin response: %w", err)
	}

	if pin.ID == 0 || strings.TrimSpace(pin.Code) == "" {
		return nil, fmt.Errorf("plex auth pin response missing required fields")
	}

	return &pin, nil
}

func (s *PlexService) CheckAuthPIN(ctx context.Context, pinID int, code, clientIdentifier, product string) (*types.PlexPIN, error) {
	if pinID <= 0 {
		return nil, fmt.Errorf("pin id is required")
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("pin code is required")
	}
	if strings.TrimSpace(clientIdentifier) == "" {
		return nil, fmt.Errorf("client identifier is required")
	}

	endpoint := fmt.Sprintf(
		"https://plex.tv/api/v2/pins/%d?code=%s",
		pinID,
		url.QueryEscape(code),
	)
	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, s.getPlexAuthHeaders(clientIdentifier, product), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query plex auth pin: %w", err)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read plex auth pin status response: %w", err)
	}

	var pin types.PlexPIN
	if err := json.Unmarshal(body, &pin); err != nil {
		return nil, fmt.Errorf("failed to parse plex auth pin status response: %w", err)
	}

	return &pin, nil
}

func (s *PlexService) GetSessions(ctx context.Context, url, apiKey string) (*types.PlexSessionsResponse, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := strings.TrimRight(url, "/")
	sessionsEndpoint := fmt.Sprintf("%s/status/sessions", baseURL)

	resp, err := s.DoRequest(ctx, http.MethodGet, sessionsEndpoint, s.getPlexHeaders(apiKey), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var sessionsResponse types.PlexSessionsResponse
	if err := json.Unmarshal(body, &sessionsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse sessions response: %v", err)
	}

	// Initialize empty slice if Metadata is nil
	if sessionsResponse.MediaContainer.Metadata == nil {
		sessionsResponse.MediaContainer.Metadata = []types.PlexSession{}
	}

	// Process each session to check for transcoding
	for i, session := range sessionsResponse.MediaContainer.Metadata {
		// Check if TranscodeSession exists and copy its details
		if session.TranscodeSession != nil {
			continue // Already has transcode info
		}

		// Initialize TranscodeSession if needed
		sessionsResponse.MediaContainer.Metadata[i].TranscodeSession = &types.PlexTranscodeSession{}

		for _, media := range session.Media {
			for _, part := range media.Part {
				if part.Decision == "transcode" {
					// Set transcoding details
					sessionsResponse.MediaContainer.Metadata[i].TranscodeSession.VideoDecision = "transcode"
					// You might also want to set other transcode details here
					break
				}
			}
		}
	}

	return &sessionsResponse, nil
}

func (s *PlexService) getVersion(ctx context.Context, url, apiKey string) (string, error) {
	healthEndpoint := s.GetHealthEndpoint(url)
	headers := s.getPlexHeaders(apiKey)

	resp, err := s.DoRequest(ctx, http.MethodGet, healthEndpoint, headers, nil)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %v", err)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var plexResponse types.PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer types.MediaContainer
		if xmlErr := xml.Unmarshal(body, &mediaContainer); xmlErr != nil {
			return "", fmt.Errorf("failed to parse server response")
		}
		plexResponse.MediaContainer = mediaContainer
	}

	// Validate version to prevent "true" being shown
	if plexResponse.MediaContainer.Version == "true" || plexResponse.MediaContainer.Version == "" {
		return "unknown", nil
	}

	return plexResponse.MediaContainer.Version, nil
}

func (s *PlexService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	healthEndpoint := s.GetHealthEndpoint(url)
	headers := s.getPlexHeaders(apiKey)

	resp, err := s.DoRequest(ctx, http.MethodGet, healthEndpoint, headers, nil)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	// Calculate response time directly
	responseTime := time.Since(startTime).Milliseconds()

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "warning", fmt.Sprintf("Failed to read response: %v", err)), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned error: %d", resp.StatusCode)), http.StatusOK
	}

	// Get version using GetCachedVersion for better caching
	version, err := s.GetCachedVersion(ctx, url, apiKey, func(baseURL, key string) (string, error) {
		return s.getVersion(ctx, baseURL, key)
	})
	if err != nil {
		version = "unknown"
	}

	var plexResponse types.PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer types.MediaContainer
		if xmlErr := xml.Unmarshal(body, &mediaContainer); xmlErr != nil {
			return s.CreateHealthResponse(startTime, "warning", "Failed to parse server response"), http.StatusOK
		}
		plexResponse.MediaContainer = mediaContainer
	}

	extras := map[string]interface{}{
		"version":         version,
		"responseTime":    responseTime,
		"updateAvailable": s.GetUpdateStatusFromCache(ctx, url), // Add update status from cache
	}

	// Always set status to "online" when healthy and include a message
	message := "Healthy"
	if plexResponse.MediaContainer.Platform != "" {
		message = fmt.Sprintf("Healthy - Running on %s", plexResponse.MediaContainer.Platform)
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}
