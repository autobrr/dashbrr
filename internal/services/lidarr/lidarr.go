// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package lidarr

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

type LidarrService struct {
	core.ServiceCore
}

func init() {
	models.NewLidarrService = NewLidarrService
}

func NewLidarrService() models.ServiceHealthChecker {
	service := &LidarrService{}
	service.Type = "lidarr"
	service.DisplayName = "Lidarr"
	service.Description = "Monitor and manage your Lidarr instance"
	service.DefaultURL = "http://localhost:8686"
	service.HealthEndpoint = "/api/v1/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *LidarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v1/health", baseURL)
}

func (s *LidarrService) DeleteQueueItem(
	ctx context.Context,
	baseURL, apiKey, queueID string,
	options types.LidarrQueueDeleteOptions,
) error {
	return arr.DeleteQueueItemWithVersion(ctx, "lidarr", "v1", baseURL, apiKey, queueID, arr.QueueDeleteOptions{
		RemoveFromClient: options.RemoveFromClient,
		Blocklist:        options.Blocklist,
		SkipRedownload:   options.SkipRedownload,
		ChangeCategory:   options.ChangeCategory,
	}, s.ReadBody)
}

func (s *LidarrService) getQueueRecords(ctx context.Context, url, apiKey string) ([]types.LidarrQueueItem, error) {
	return arr.FetchQueueRecordsWithVersion[types.LidarrQueueItem](
		ctx,
		"lidarr",
		"v1",
		url,
		apiKey,
		"page=1&pageSize=10&includeUnknownArtistItems=false&includeArtist=true&includeAlbum=true",
		s.ReadBody,
	)
}

func (s *LidarrService) GetQueue(ctx context.Context, url, apiKey string) (any, error) {
	records, err := s.getQueueRecords(ctx, url, apiKey)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *LidarrService) GetQueueForHealth(ctx context.Context, url, apiKey string) ([]types.LidarrQueueItem, error) {
	return s.getQueueRecords(ctx, url, apiKey)
}

func (s *LidarrService) GetSystemStatus(ctx context.Context, url, apiKey string) (string, error) {
	return arr.GetArrSystemStatusWithVersion(ctx, "lidarr", "v1", url, apiKey, s.GetVersionFromCache, s.CacheVersion)
}

func (s *LidarrService) CheckForUpdates(ctx context.Context, url, apiKey string) (bool, error) {
	return arr.CheckArrForUpdatesWithVersion(ctx, "lidarr", "v1", url, apiKey)
}

func (s *LidarrService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	return arr.ArrHealthCheck(ctx, &s.ServiceCore, url, apiKey, s)
}
