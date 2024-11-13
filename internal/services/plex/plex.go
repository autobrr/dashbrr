// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package plex

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
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

func (s *PlexService) GetSessions(url, apiKey string) (*types.PlexSessionsResponse, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

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

func (s *PlexService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultTimeout)
	defer cancel()

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

	var plexResponse types.PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer types.MediaContainer
		if xmlErr := xml.Unmarshal(body, &mediaContainer); xmlErr != nil {
			return s.CreateHealthResponse(startTime, "warning", "Failed to parse server response"), http.StatusOK
		}
		plexResponse.MediaContainer = mediaContainer
	}

	extras := map[string]interface{}{
		"version":      plexResponse.MediaContainer.Version,
		"responseTime": responseTime,
	}

	// Always set status to "online" when healthy and include a message
	message := "Healthy"
	if plexResponse.MediaContainer.Platform != "" {
		message = fmt.Sprintf("Healthy - Running on %s", plexResponse.MediaContainer.Platform)
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}
