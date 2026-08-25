// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package whisparr

import (
	"context"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
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

func (s *WhisparrService) DeleteQueueItem(
	ctx context.Context,
	baseURL, apiKey, queueID string,
	options types.WhisparrQueueDeleteOptions,
) error {
	return arr.DeleteQueueItem(ctx, "whisparr", baseURL, apiKey, queueID, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *WhisparrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.WhisparrQueueItem, error) {
	return arr.FetchQueueRecords[types.WhisparrQueueItem](
		ctx,
		"whisparr",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownSeriesItems=false&includeSeries=true&includeEpisode=true",
		s.ReadBody,
	)
}

func (s *WhisparrService) GetQueue(ctx context.Context, url, apiKey string) (any, error) {
	records, err := s.getQueueRecords(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *WhisparrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.WhisparrQueueItem, error) {
	return s.getQueueRecords(ctx, url, apiKey)
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
