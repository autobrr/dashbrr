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
	"github.com/autobrr/dashbrr/internal/services/base"
)

type PlexService struct {
	base.BaseService
}

type MediaContainer struct {
	XMLName  xml.Name `xml:"MediaContainer"`
	Version  string   `xml:"version,attr"`
	Platform string   `xml:"platform,attr"`
}

type PlexResponse struct {
	MediaContainer MediaContainer `json:"MediaContainer"`
}

type PlexSessionsResponse struct {
	MediaContainer struct {
		Size     int           `json:"size"`
		Metadata []PlexSession `json:"Metadata"`
	} `json:"MediaContainer"`
}

type PlexUser struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Thumb string `json:"thumb"`
}

type PlexPlayer struct {
	Address             string `json:"address"`
	Device              string `json:"device"`
	MachineIdentifier   string `json:"machineIdentifier"`
	Model               string `json:"model"`
	Platform            string `json:"platform"`
	PlatformVersion     string `json:"platformVersion"`
	Product             string `json:"product"`
	Profile             string `json:"profile"`
	State               string `json:"state"`
	RemotePublicAddress string `json:"remotePublicAddress"`
	Title               string `json:"title"`
	Vendor              string `json:"vendor"`
	Version             string `json:"version"`
}

type PlexSession struct {
	Title            string      `json:"title"`
	GrandparentTitle string      `json:"grandparentTitle"`
	Type             string      `json:"type"`
	User             *PlexUser   `json:"User,omitempty"`
	Player           *PlexPlayer `json:"Player,omitempty"`
	State            string      `json:"state"`
	TranscodeSession *struct {
		VideoDecision string  `json:"videoDecision"`
		AudioDecision string  `json:"audioDecision"`
		Progress      float64 `json:"progress"`
	} `json:"TranscodeSession,omitempty"`
}

const sessionCacheDuration = 30 * time.Second

func init() {
	models.NewPlexService = func() models.ServiceHealthChecker {
		return &PlexService{}
	}
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

func (s *PlexService) GetSessions(url, apiKey string) (*PlexSessionsResponse, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	var sessionsResponse PlexSessionsResponse
	if err := json.Unmarshal(body, &sessionsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse sessions response: %v", err)
	}

	// Initialize empty slice if Metadata is nil
	if sessionsResponse.MediaContainer.Metadata == nil {
		sessionsResponse.MediaContainer.Metadata = []PlexSession{}
	}

	return &sessionsResponse, nil
}

func (s *PlexService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthEndpoint := s.GetHealthEndpoint(url)
	headers := s.getPlexHeaders(apiKey)

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "warning", fmt.Sprintf("Failed to read response: %v", err)), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned error: %d", resp.StatusCode)), http.StatusOK
	}

	var plexResponse PlexResponse
	if err := json.Unmarshal(body, &plexResponse); err != nil {
		var mediaContainer MediaContainer
		if xmlErr := xml.Unmarshal(body, &mediaContainer); xmlErr != nil {
			return s.CreateHealthResponse(startTime, "warning", "Failed to parse server response"), http.StatusOK
		}
		plexResponse.MediaContainer = mediaContainer
	}

	extras := map[string]interface{}{
		"version":      plexResponse.MediaContainer.Version,
		"responseTime": responseTime.Milliseconds(),
	}

	message := fmt.Sprintf("Running on %s", plexResponse.MediaContainer.Platform)
	if plexResponse.MediaContainer.Platform == "" {
		message = "Healthy"
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}
