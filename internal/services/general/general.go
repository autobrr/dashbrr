// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

func init() {
	models.NewGeneralService = NewGeneralService
}

func NewGeneralService() models.ServiceHealthChecker {
	service := &GeneralService{}
	service.Type = "general"
	service.DisplayName = "" // Allow display name to be set via configuration
	service.Description = "Generic health check service for any URL endpoint"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

// GeneralService is the models.ServiceHealthChecker implementation registered
// for the "general" service type. All of the real work lives in Engine, which
// additionally knows how to drive a models.CustomServiceConfig (auth, login,
// health, stats, actions). GeneralService.CheckHealth keeps the exact 3-arg
// signature the poller/registry call through models.ServiceHealthChecker, and
// delegates to Engine with a nil config so behaviour for services with no
// custom config is unchanged.
type GeneralService struct {
	Engine
}

func (s *GeneralService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return s.Engine.CheckHealth(ctx, url, apiKey, nil)
}

func (s *GeneralService) GetVersion(ctx context.Context, url, apiKey string) (string, error) {
	return "", nil // Version not supported for general service
}

func (s *GeneralService) GetLatestVersion(ctx context.Context) (string, error) {
	return "", nil // Version not supported for general service
}
