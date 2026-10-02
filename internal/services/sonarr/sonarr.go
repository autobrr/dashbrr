// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sonarr

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type SonarrService struct {
	core.ServiceCore
}

func init() {
	models.NewSonarrService = NewSonarrService
}

func NewSonarrService() models.ServiceHealthChecker {
	service := &SonarrService{}
	service.Type = "sonarr"
	service.DisplayName = "Sonarr"
	service.Description = "Monitor and manage your Sonarr instance"
	service.DefaultURL = "http://localhost:8989"
	service.HealthEndpoint = "/api/v3/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *SonarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

// DeleteQueueItem deletes a queue item with the specified options
func (s *SonarrService) DeleteQueueItem(ctx context.Context, baseURL, apiKey string, queueId string, options types.SonarrQueueDeleteOptions) error {
	return arr.DeleteQueueItem(ctx, "sonarr", baseURL, apiKey, queueId, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *SonarrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.QueueRecord, error) {
	records, err := arr.FetchQueueRecords[types.QueueRecord](
		ctx,
		"sonarr",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownSeriesItems=false&includeSeries=true&includeEpisode=true",
		s.ReadBody,
	)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetQueueForHealth is a wrapper around GetQueue that returns []types.QueueRecord
func (s *SonarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.QueueRecord, error) {
	return s.getQueueRecords(ctx, url, apiKey)
}

// GetSystemStatus fetches the system status from Sonarr
func (s *SonarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatus(ctx, "sonarr", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

// CheckForUpdates checks if there are any updates available for Sonarr
func (s *SonarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdates(ctx, "sonarr", url, apiKey)
}

func (s *SonarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
