// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package readarr

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type ReadarrService struct {
	core.ServiceCore
}

func init() {
	models.NewReadarrService = NewReadarrService
}

func NewReadarrService() models.ServiceHealthChecker {
	service := &ReadarrService{}
	service.Type = "readarr"
	service.DisplayName = "Readarr"
	service.Description = "Monitor and manage your Readarr instance"
	service.DefaultURL = "http://localhost:8787"
	service.HealthEndpoint = "/api/v1/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *ReadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/health", baseURL)
}

func (s *ReadarrService) DeleteQueueItem(
	ctx context.Context,
	baseURL, apiKey, queueID string,
	options types.ReadarrQueueDeleteOptions,
) error {
	return arr.DeleteQueueItemWithVersion(ctx, "readarr", "v1", baseURL, apiKey, queueID, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *ReadarrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.ReadarrQueueItem, error) {
	return arr.FetchQueueRecordsWithVersion[types.ReadarrQueueItem](
		ctx,
		"readarr",
		"v1",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownAuthorItems=false&includeAuthor=true&includeBook=true",
		s.ReadBody,
	)
}

func (s *ReadarrService) GetQueue(ctx context.Context, url, apiKey string) (interface{}, error) {
	records, err := s.getQueueRecords(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *ReadarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.ReadarrQueueItem, error) {
	return s.getQueueRecords(ctx, url, apiKey)
}

func (s *ReadarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatusWithVersion(ctx, "readarr", "v1", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

func (s *ReadarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdatesWithVersion(ctx, "readarr", "v1", url, apiKey)
}

func (s *ReadarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
