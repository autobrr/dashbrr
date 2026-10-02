// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package whisparr

import (
	"context"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
)

//nolint:revive // named for consistency with every sibling *arr service (SonarrService, LidarrService, ...)
type WhisparrService struct {
	core.ServiceCore
}

func init() {
	models.NewWhisparrService = NewWhisparrService
}

func NewWhisparrService() models.ServiceHealthChecker {
	service := &WhisparrService{}
	service.Type = "whisparr"
	service.DisplayName = "Whisparr"
	service.Description = "Monitor and manage your Whisparr instance"
	service.DefaultURL = "http://localhost:6969"
	service.HealthEndpoint = "/api/v3/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *WhisparrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return baseURL + "/api/v3/health"
}

func (s *WhisparrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatus(ctx, "whisparr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

func (s *WhisparrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "whisparr", url, apiKey)
}

func (s *WhisparrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
