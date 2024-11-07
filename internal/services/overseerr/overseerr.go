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

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

type OverseerrService struct {
	core.ServiceCore
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

func init() {
	models.NewOverseerrService = func() models.ServiceHealthChecker {
		return &OverseerrService{}
	}
}

func (s *OverseerrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/status", baseURL)
}

func (s *OverseerrService) GetPendingRequests(url, apiKey string) (int, error) {
	if url == "" {
		return 0, fmt.Errorf("URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(url, "/")
	requestEndpoint := fmt.Sprintf("%s/api/v1/request?filter=pending", baseURL)

	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, requestEndpoint, "", headers)
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("server returned error: %d", resp.StatusCode)
	}

	var requestsResponse RequestsResponse
	if err := json.Unmarshal(body, &requestsResponse); err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	return requestsResponse.PageInfo.Results, nil
}

func (s *OverseerrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
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
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	// Get response time from header
	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "warning", fmt.Sprintf("Failed to read response: %v", err)), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned error: %d", resp.StatusCode)), http.StatusOK
	}

	// Parse the response
	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return s.CreateHealthResponse(startTime, "warning", "Failed to parse status response"), http.StatusOK
	}

	// Create response with version, update information, and response time
	extras := map[string]interface{}{
		"version":         statusResponse.Version,
		"updateAvailable": statusResponse.UpdateAvailable,
		"responseTime":    responseTime.Milliseconds(),
	}

	status := "online"
	message := "Healthy"

	if statusResponse.Status != 0 {
		if statusResponse.Status >= 400 {
			status = "warning"
			message = fmt.Sprintf("Service reported status code: %d", statusResponse.Status)
		}
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(url, statusResponse.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return s.CreateHealthResponse(startTime, status, message, extras), http.StatusOK
}
