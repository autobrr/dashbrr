// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"fmt"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/types"
)

func buildPlexSessionsServiceUpdate(instanceID string, sessions []types.PlexSession) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "plex_sessions",
		Stats: map[string]interface{}{
			"plex": map[string]interface{}{
				"sessions": sessions,
			},
		},
		Details: map[string]interface{}{
			"plex": map[string]interface{}{
				"activeStreams": len(sessions),
				"transcoding":   countTranscodingSessions(sessions),
			},
		},
	}
}

func buildOverseerrRequestsServiceUpdate(instanceID string, stats *types.RequestsStats) models.ServiceHealth {
	serviceStatus := "online"
	if stats.PendingCount > 0 {
		serviceStatus = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    serviceStatus,
		Message:   "overseerr_requests",
		Stats: map[string]interface{}{
			"overseerr": types.OverseerrStats{
				Requests:     stats.Requests,
				PendingCount: stats.PendingCount,
			},
		},
		Details: map[string]interface{}{
			"overseerr": types.OverseerrDetails{
				PendingCount:  stats.PendingCount,
				TotalRequests: len(stats.Requests),
			},
		},
	}
}

func buildRadarrQueueServiceUpdate(instanceID string, queueResp *types.RadarrQueueResponse) models.ServiceHealth {
	downloading, totalSize := summarizeRadarrQueue(queueResp.Records)

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "radarr_queue",
		Stats: map[string]interface{}{
			"radarr": map[string]interface{}{
				"queue": queueResp,
			},
		},
		Details: map[string]interface{}{
			"radarr": map[string]interface{}{
				"queueCount":       queueResp.TotalRecords,
				"totalRecords":     queueResp.TotalRecords,
				"downloadingCount": downloading,
				"totalSize":        totalSize,
			},
		},
	}
}

func buildLidarrQueueServiceUpdate(instanceID string, queueResp *types.LidarrQueueResponse) models.ServiceHealth {
	downloading, totalSize := summarizeLidarrQueue(queueResp.Records)

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "lidarr_queue",
		Stats: map[string]interface{}{
			"lidarr": map[string]interface{}{
				"queue": queueResp,
			},
		},
		Details: map[string]interface{}{
			"lidarr": map[string]interface{}{
				"queueCount":       queueResp.TotalRecords,
				"totalRecords":     queueResp.TotalRecords,
				"downloadingCount": downloading,
				"totalSize":        totalSize,
			},
		},
	}
}

func buildSonarrQueueServiceUpdate(instanceID string, queueResp *types.SonarrQueueResponse) models.ServiceHealth {
	downloading, episodeCount, totalSize := summarizeSonarrQueue(queueResp.Records)

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "sonarr_queue",
		Stats: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queue": queueResp,
			},
		},
		Details: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queueCount":       queueResp.TotalRecords,
				"totalRecords":     queueResp.TotalRecords,
				"downloadingCount": downloading,
				"episodeCount":     episodeCount,
				"totalSize":        totalSize,
			},
		},
	}
}

func buildSonarrStatsServiceUpdate(instanceID string, statsResp *types.SonarrStatsResponse, version string) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "sonarr_stats",
		Stats: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"stats":   statsResp,
				"version": version,
			},
		},
		Details: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"monitored":  statsResp.Monitored,
				"version":    version,
				"queueCount": statsResp.QueuedCount,
			},
		},
	}
}

func buildProwlarrStatsServiceUpdate(instanceID string, stats types.ProwlarrStatsResponse) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "prowlarr_stats",
		Stats: map[string]interface{}{
			"prowlarr": map[string]interface{}{
				"stats": stats,
			},
		},
	}
}

func buildProwlarrIndexersServiceUpdate(instanceID string, indexers []types.ProwlarrIndexer) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "prowlarr_indexers",
		Stats: map[string]interface{}{
			"prowlarr": map[string]interface{}{
				"indexers": indexers,
			},
		},
	}
}

func buildAutobrrStatsServiceUpdate(instanceID string, stats types.AutobrrStats) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "autobrr_stats",
		Stats: map[string]interface{}{
			"autobrr": map[string]interface{}{
				"stats": stats,
			},
		},
	}
}

func buildAutobrrReleasesServiceUpdate(instanceID string, releases types.ReleasesResponse) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "autobrr_releases",
		Stats: map[string]interface{}{
			"autobrr": map[string]interface{}{
				"releases": releases,
			},
		},
	}
}

func buildAutobrrIRCServiceUpdate(instanceID string, status []types.IRCStatus) (models.ServiceHealth, models.ServiceEventType) {
	health := models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "autobrr_irc_status",
		Details: map[string]interface{}{
			"autobrr": types.AutobrrDetails{
				IRC: status,
			},
		},
	}

	for _, irc := range status {
		if !irc.Healthy && irc.Enabled {
			health.Status = "warning"
			health.Message = fmt.Sprintf("IRC network %s is unhealthy", irc.Name)
			return health, models.ServiceEventHealth
		}
	}

	return health, models.ServiceEventInternal
}

func buildMaintainerrCollectionsServiceUpdate(instanceID string, collections []maintainerr.Collection) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "maintainerr_collections",
		Stats: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collections": collections,
			},
		},
		Details: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collectionCount": len(collections),
			},
		},
	}
}

func buildTailscaleDevicesServiceUpdate(instanceID string, devices []tailscale.Device) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "tailscale_devices",
		Stats: map[string]interface{}{
			"tailscale": map[string]interface{}{
				"devices": devices,
			},
		},
		Details: map[string]interface{}{
			"tailscale": map[string]interface{}{
				"total":  len(devices),
				"online": countOnlineDevices(devices),
			},
		},
	}
}

func buildQuiOverviewServiceUpdate(instanceID string, instances []types.QuiInstance, summary types.QuiTransferSummary, transfers []types.QuiInstanceTransfer) models.ServiceHealth {
	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    summarizeQuiCardStatus(summary),
		Message:   "qui_overview",
		Stats: map[string]interface{}{
			"qui": map[string]interface{}{
				"instances": instances,
				"transfers": transfers,
			},
		},
		Details: map[string]interface{}{
			"qui": map[string]interface{}{
				"summary": summary,
			},
		},
	}
}
