// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package radarr

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
)

type RadarrService struct {
	core.ServiceCore
}

func NewRadarrService() models.ServiceHealthChecker {
	service := &RadarrService{}
	service.Type = "radarr"
	service.DisplayName = "Radarr"
	service.Description = "Monitor and manage your Radarr instance"
	service.DefaultURL = "http://localhost:7878"
	service.HealthEndpoint = "/api/v3/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *RadarrService) GetHealthEndpoint(baseURL string) (string, error) {
	return core.RequestURL(baseURL, "/api/v3/health")
}

// GetSystemStatus fetches the system status from Radarr
func (s *RadarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatus(ctx, "radarr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Radarr
func (s *RadarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "radarr", url, apiKey)
}

func (s *RadarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
