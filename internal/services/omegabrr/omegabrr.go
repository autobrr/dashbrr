// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package omegabrr

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

type OmegabrrService struct {
	core.ServiceCore
}

type VersionResponse struct {
	Version string `json:"version"`
}

func init() {
	models.NewOmegabrrService = func() models.ServiceHealthChecker {
		return &OmegabrrService{}
	}
}

func NewOmegabrrService() models.ServiceHealthChecker {
	service := &OmegabrrService{}
	service.Type = "omegabrr"
	service.DisplayName = "Omegabrr"
	service.Description = "Monitor and manage your Omegabrr instance"
	service.DefaultURL = "http://localhost:7474"
	service.HealthEndpoint = "/api/healthz/liveness"
	service.SetTimeout(core.DefaultLongTimeout) // Set longer timeout for Omegabrr
	return service
}

func (s *OmegabrrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/healthz/liveness", baseURL)
}

func (s *OmegabrrService) GetVersionEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/version", baseURL)
}

func (s *OmegabrrService) getVersion(ctx context.Context, url, apiKey string) (string, error) {
	versionEndpoint := s.GetVersionEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, versionEndpoint, "", headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", err
	}

	var version VersionResponse
	if err := json.Unmarshal(body, &version); err != nil {
		return "", err
	}

	// Validate version to prevent "true" being shown
	if version.Version == "true" || version.Version == "" {
		return "unknown", nil
	}

	return version.Version, nil
}

func (s *OmegabrrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	// Create a child context with longer timeout if needed
	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultLongTimeout)
	defer cancel()

	// Get version using GetCachedVersion for better caching
	version, err := s.GetCachedVersion(healthCtx, url, apiKey, func(baseURL, key string) (string, error) {
		return s.getVersion(healthCtx, baseURL, key)
	})

	// Check health endpoint
	healthEndpoint := s.GetHealthEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(healthCtx, healthEndpoint, "", headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", "Failed to connect: "+err.Error()), http.StatusOK
	}
	defer resp.Body.Close()

	// Calculate response time directly
	responseTime := time.Since(startTime).Milliseconds()

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "warning", "Failed to read response: "+err.Error()), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned error: %d", resp.StatusCode)), http.StatusOK
	}

	if strings.TrimSpace(string(body)) != "OK" {
		return s.CreateHealthResponse(startTime, "warning", "Unexpected response from server"), http.StatusOK
	}

	extras := map[string]interface{}{
		"responseTime": responseTime,
	}

	if version != "" {
		extras["version"] = version
		// Check update status from cache
		extras["updateAvailable"] = s.GetUpdateStatusFromCache(url)
	}

	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}

// TriggerARRsWebhook triggers the ARRs webhook
func (s *OmegabrrService) TriggerARRsWebhook(ctx context.Context, url, apiKey string) int {
	if url == "" {
		return http.StatusBadRequest
	}

	webhookEndpoint := fmt.Sprintf("%s/api/webhook/trigger/arr?apikey=%s", strings.TrimRight(url, "/"), apiKey)
	resp, err := s.MakeRequestWithContext(ctx, webhookEndpoint, "", nil)
	if err != nil {
		return http.StatusInternalServerError
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

// TriggerListsWebhook triggers the Lists webhook
func (s *OmegabrrService) TriggerListsWebhook(ctx context.Context, url, apiKey string) int {
	if url == "" {
		return http.StatusBadRequest
	}

	webhookEndpoint := fmt.Sprintf("%s/api/webhook/trigger/lists?apikey=%s", strings.TrimRight(url, "/"), apiKey)
	resp, err := s.MakeRequestWithContext(ctx, webhookEndpoint, "", nil)
	if err != nil {
		return http.StatusInternalServerError
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

// TriggerAllWebhooks triggers all webhooks
func (s *OmegabrrService) TriggerAllWebhooks(ctx context.Context, url, apiKey string) int {
	if url == "" {
		return http.StatusBadRequest
	}

	webhookEndpoint := fmt.Sprintf("%s/api/webhook/trigger?apikey=%s", strings.TrimRight(url, "/"), apiKey)
	resp, err := s.MakeRequestWithContext(ctx, webhookEndpoint, "", nil)
	if err != nil {
		return http.StatusInternalServerError
	}
	defer resp.Body.Close()

	return resp.StatusCode
}
