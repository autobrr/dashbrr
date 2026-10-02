// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package radarr

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type RadarrService struct {
	core.ServiceCore
}

func init() {
	models.NewRadarrService = NewRadarrService
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

func (s *RadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *RadarrService) DeleteQueueItem(ctx context.Context, baseURL, apiKey string, queueId string, options types.RadarrQueueDeleteOptions) error {
	return arr.DeleteQueueItem(ctx, "radarr", baseURL, apiKey, queueId, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *RadarrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.RadarrQueueRecord, error) {
	records, err := arr.FetchQueueRecords[types.RadarrQueueRecord](
		ctx,
		"radarr",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownMovieItems=false&includeMovie=false",
		s.ReadBody,
	)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []types.RadarrQueueRecord
func (s *RadarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.RadarrQueueRecord, error) {
	return s.getQueueRecords(ctx, url, apiKey)
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
