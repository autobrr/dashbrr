// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"
)

type PlexService struct {
	ServiceCore
}

func NewPlexService(db *database.DB, cache cache.Store, config *domain.ServiceConfiguration) ServiceHealthChecker {
	service := &PlexService{
		ServiceCore: ServiceCore{
			Type:           domain.ServiceTypePlex,
			DisplayName:    config.DisplayName,
			Description:    "Monitor and manage your Plex MaintainerrMedia Server",
			DefaultURL:     "http://localhost:32400",
			HealthEndpoint: "/identity",
			URL:            config.URL,
			ApiKey:         config.APIKey,
			InstanceID:     config.InstanceID,
		},
	}
	service.SetTimeout(DefaultTimeout)
	service.SetDB(db)
	service.SetCache(cache)
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

func (s *PlexService) GetSessions(ctx context.Context, url, apiKey string) (*domain.PlexSessionsResponse, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := strings.TrimRight(url, "/")
	sessionsEndpoint := fmt.Sprintf("%s/status/sessions", baseURL)

	resp, err := s.MakeRequestWithContext(ctx, sessionsEndpoint, "", s.getPlexHeaders(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var sessionsResponse domain.PlexSessionsResponse
	if err := json.Unmarshal(body, &sessionsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse sessions response: %v", err)
	}

	// Initialize empty slice if Metadata is nil
	if sessionsResponse.MediaContainer.Metadata == nil {
		sessionsResponse.MediaContainer.Metadata = []domain.PlexSession{}
	}

	// Process each session to check for transcoding
	for i, session := range sessionsResponse.MediaContainer.Metadata {
		// Check if TranscodeSession exists and copy its details
		if session.TranscodeSession != nil {
			continue // Already has transcode info
		}

		// Initialize TranscodeSession if needed
		sessionsResponse.MediaContainer.Metadata[i].TranscodeSession = &domain.PlexTranscodeSession{}

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

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var plexResponse domain.PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer domain.MediaContainer
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

func (s *PlexService) CheckHealth(ctx context.Context, url, apiKey string) (*domain.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	healthEndpoint := s.GetHealthEndpoint(url)
	headers := s.getPlexHeaders(apiKey)

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
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

	var plexResponse domain.PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer domain.MediaContainer
		if xmlErr := xml.Unmarshal(body, &mediaContainer); xmlErr != nil {
			return s.CreateHealthResponse(startTime, "warning", "Failed to parse server response"), http.StatusOK
		}
		plexResponse.MediaContainer = mediaContainer
	}

	extras := map[string]interface{}{
		"version":         version,
		"responseTime":    responseTime,
		"updateAvailable": s.GetUpdateStatusFromCache(url), // Add update status from cache
	}

	// Always set status to "online" when healthy and include a message
	message := "Healthy"
	if plexResponse.MediaContainer.Platform != "" {
		message = fmt.Sprintf("Healthy - Running on %s", plexResponse.MediaContainer.Platform)
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}
