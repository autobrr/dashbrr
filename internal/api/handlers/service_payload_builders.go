// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"fmt"
	"strconv"
	"strings"

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

func buildJellyfinSummaryServiceUpdate(instanceID string, summary *types.JellyfinSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.JellyfinSummaryResponse{
			Sessions: []types.JellyfinSession{},
		}
	}
	if summary.Sessions == nil {
		summary.Sessions = []types.JellyfinSession{}
	}

	activeStreams := len(summary.Sessions)
	transcoding := countJellyfinTranscoding(summary.Sessions)
	paused := countJellyfinPaused(summary.Sessions)

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "jellyfin_summary",
		Stats: map[string]interface{}{
			"jellyfin": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"jellyfin": map[string]interface{}{
				"activeStreams": activeStreams,
				"transcoding":   transcoding,
				"paused":        paused,
				"serverName":    summary.System.ServerName,
			},
		},
	}
}

func buildUptimeKumaSummaryServiceUpdate(instanceID string, summary *types.UptimeKumaSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.UptimeKumaSummaryResponse{
			Monitors: []types.UptimeKumaMonitor{},
		}
	}
	if summary.Monitors == nil {
		summary.Monitors = []types.UptimeKumaMonitor{}
	}

	total, up, down, pending, maintenance := countUptimeKumaStates(summary.Monitors)
	status := "online"
	if down > 0 || pending > 0 {
		status = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    status,
		Message:   "uptimekuma_summary",
		Stats: map[string]interface{}{
			"uptimekuma": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"uptimekuma": map[string]interface{}{
				"total":       total,
				"up":          up,
				"down":        down,
				"pending":     pending,
				"maintenance": maintenance,
				"issues":      down + pending,
			},
		},
	}
}

func buildTraefikSummaryServiceUpdate(instanceID string, summary *types.TraefikSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.TraefikSummaryResponse{
			IssueRouters: []types.TraefikRouter{},
		}
	}
	if summary.IssueRouters == nil {
		summary.IssueRouters = []types.TraefikRouter{}
	}

	routerTotal := sectionTotal(summary.Overview.HTTP.Routers) + sectionTotal(summary.Overview.TCP.Routers) + sectionTotal(summary.Overview.UDP.Routers)
	routerWarnings := sectionWarnings(summary.Overview.HTTP.Routers) + sectionWarnings(summary.Overview.TCP.Routers) + sectionWarnings(summary.Overview.UDP.Routers)
	routerErrors := sectionErrors(summary.Overview.HTTP.Routers) + sectionErrors(summary.Overview.TCP.Routers) + sectionErrors(summary.Overview.UDP.Routers)

	serviceTotal := sectionTotal(summary.Overview.HTTP.Services) + sectionTotal(summary.Overview.TCP.Services) + sectionTotal(summary.Overview.UDP.Services)
	serviceWarnings := sectionWarnings(summary.Overview.HTTP.Services) + sectionWarnings(summary.Overview.TCP.Services) + sectionWarnings(summary.Overview.UDP.Services)
	serviceErrors := sectionErrors(summary.Overview.HTTP.Services) + sectionErrors(summary.Overview.TCP.Services) + sectionErrors(summary.Overview.UDP.Services)

	middlewareTotal := sectionTotal(summary.Overview.HTTP.Middlewares) + sectionTotal(summary.Overview.TCP.Middlewares)
	middlewareWarnings := sectionWarnings(summary.Overview.HTTP.Middlewares) + sectionWarnings(summary.Overview.TCP.Middlewares)
	middlewareErrors := sectionErrors(summary.Overview.HTTP.Middlewares) + sectionErrors(summary.Overview.TCP.Middlewares)

	certificateTotal := 0
	certificateExpired := 0
	certificateExpiringSoon := 0
	certificateNextExpiry := ""
	certificateNextExpiryInSeconds := int64(0)
	if summary.Certificates != nil {
		certificateTotal = summary.Certificates.Total
		certificateExpired = summary.Certificates.Expired
		certificateExpiringSoon = summary.Certificates.ExpiringSoon
		certificateNextExpiry = summary.Certificates.NextExpiry
		certificateNextExpiryInSeconds = summary.Certificates.NextExpiryInSeconds
	}

	status := "online"
	if routerWarnings+routerErrors+serviceWarnings+serviceErrors+middlewareWarnings+middlewareErrors+certificateExpired+certificateExpiringSoon > 0 {
		status = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    status,
		Message:   "traefik_summary",
		Stats: map[string]interface{}{
			"traefik": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"traefik": map[string]interface{}{
				"routerTotal":                    routerTotal,
				"routerWarnings":                 routerWarnings,
				"routerErrors":                   routerErrors,
				"serviceTotal":                   serviceTotal,
				"serviceWarnings":                serviceWarnings,
				"serviceErrors":                  serviceErrors,
				"middlewareTotal":                middlewareTotal,
				"middlewareWarnings":             middlewareWarnings,
				"middlewareErrors":               middlewareErrors,
				"providers":                      len(summary.Overview.Providers),
				"issueRouters":                   len(summary.IssueRouters),
				"metrics":                        summary.Overview.Features.Metrics,
				"tracing":                        summary.Overview.Features.Tracing,
				"accessLog":                      summary.Overview.Features.AccessLog,
				"certificateTotal":               certificateTotal,
				"certificateExpired":             certificateExpired,
				"certificateExpiringSoon":        certificateExpiringSoon,
				"certificateNextExpiry":          certificateNextExpiry,
				"certificateNextExpiryInSeconds": certificateNextExpiryInSeconds,
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

func sectionTotal(section *types.TraefikSection) int {
	if section == nil {
		return 0
	}
	return section.Total
}

func sectionWarnings(section *types.TraefikSection) int {
	if section == nil {
		return 0
	}
	return section.Warnings
}

func sectionErrors(section *types.TraefikSection) int {
	if section == nil {
		return 0
	}
	return section.Errors
}

func countJellyfinTranscoding(sessions []types.JellyfinSession) int {
	transcoding := 0
	for _, session := range sessions {
		if session.TranscodingInfo != nil {
			transcoding++
		}
	}
	return transcoding
}

func countJellyfinPaused(sessions []types.JellyfinSession) int {
	paused := 0
	for _, session := range sessions {
		if session.PlayState != nil && session.PlayState.IsPaused {
			paused++
		}
	}
	return paused
}

func countUptimeKumaStates(monitors []types.UptimeKumaMonitor) (total, up, down, pending, maintenance int) {
	total = len(monitors)
	for _, monitor := range monitors {
		switch monitor.Status {
		case "up":
			up++
		case "down":
			down++
		case "pending":
			pending++
		case "maintenance":
			maintenance++
		}
	}
	return total, up, down, pending, maintenance
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

func buildReadarrQueueServiceUpdate(instanceID string, queueResp *types.ReadarrQueueResponse) models.ServiceHealth {
	downloading, totalSize := summarizeReadarrQueue(queueResp.Records)

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    "online",
		Message:   "readarr_queue",
		Stats: map[string]interface{}{
			"readarr": map[string]interface{}{
				"queue": queueResp,
			},
		},
		Details: map[string]interface{}{
			"readarr": map[string]interface{}{
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

func buildBazarrSummaryServiceUpdate(instanceID string, summary *types.BazarrSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.BazarrSummaryResponse{
			Providers:    []types.BazarrProviderStatus{},
			HealthIssues: []types.BazarrHealthIssue{},
		}
	}

	status := "online"
	if len(summary.HealthIssues) > 0 || len(summary.Providers) > 0 || summary.Badges.Status > 0 {
		status = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    status,
		Message:   "bazarr_summary",
		Stats: map[string]interface{}{
			"bazarr": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"bazarr": map[string]interface{}{
				"episodeBacklog":      summary.Badges.Episodes,
				"movieBacklog":        summary.Badges.Movies,
				"providersWithIssues": len(summary.Providers),
				"healthIssues":        len(summary.HealthIssues),
				"sonarrSignalR":       summary.Badges.SonarrSignalR,
				"radarrSignalR":       summary.Badges.RadarrSignalR,
			},
		},
	}
}

func buildSabnzbdSummaryServiceUpdate(instanceID string, summary *types.SabnzbdSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.SabnzbdSummaryResponse{
			Queue: types.SabnzbdQueue{
				Slots: []types.SabnzbdQueueSlot{},
			},
			RecentFailures: []types.SabnzbdHistorySlot{},
		}
	}
	if summary.Queue.Slots == nil {
		summary.Queue.Slots = []types.SabnzbdQueueSlot{}
	}
	if summary.RecentFailures == nil {
		summary.RecentFailures = []types.SabnzbdHistorySlot{}
	}

	queueCount := parseSummaryCount(summary.Queue.NoOfSlots)
	totalQueueCount := parseSummaryCount(summary.Queue.NoOfSlotsTotal)
	if totalQueueCount <= 0 {
		totalQueueCount = queueCount
	}
	warningsCount := parseSummaryCount(summary.Queue.HaveWarnings)

	status := "online"
	if warningsCount > 0 || summary.FailedCount > 0 || strings.EqualFold(summary.Queue.Status, "paused") {
		status = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    status,
		Message:   "sabnzbd_summary",
		Stats: map[string]interface{}{
			"sabnzbd": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"sabnzbd": map[string]interface{}{
				"queueCount":       queueCount,
				"totalQueueCount":  totalQueueCount,
				"failedCount":      summary.FailedCount,
				"warningsCount":    warningsCount,
				"status":           summary.Queue.Status,
				"speed":            summary.Queue.Speed,
				"timeLeft":         summary.Queue.TimeLeft,
				"sizeLeft":         summary.Queue.SizeLeft,
				"incompleteFree":   summary.Queue.Diskspace1Norm,
				"completeFree":     summary.Queue.Diskspace2Norm,
				"recentFailureLen": len(summary.RecentFailures),
			},
		},
	}
}

func buildNzbgetSummaryServiceUpdate(instanceID string, summary *types.NzbgetSummaryResponse) models.ServiceHealth {
	if summary == nil {
		summary = &types.NzbgetSummaryResponse{
			Queue:          []types.NzbgetQueueItem{},
			RecentFailures: []types.NzbgetHistoryItem{},
		}
	}
	if summary.Queue == nil {
		summary.Queue = []types.NzbgetQueueItem{}
	}
	if summary.RecentFailures == nil {
		summary.RecentFailures = []types.NzbgetHistoryItem{}
	}

	queueCount := len(summary.Queue)
	paused := summary.Status.DownloadPaused
	downloadRate := joinHiLo(summary.Status.DownloadRateHi, summary.Status.DownloadRateLo)
	if downloadRate == 0 && summary.Status.DownloadRate > 0 {
		downloadRate = uint64(summary.Status.DownloadRate)
	}
	remainingBytes := joinHiLo(summary.Status.RemainingSizeHi, summary.Status.RemainingSizeLo)
	freeBytes := joinHiLo(summary.Status.FreeDiskSpaceHi, summary.Status.FreeDiskSpaceLo)

	status := "online"
	if paused || summary.FailedCount > 0 || summary.Status.QuotaReached {
		status = "warning"
	}

	return models.ServiceHealth{
		ServiceID: instanceID,
		Status:    status,
		Message:   "nzbget_summary",
		Stats: map[string]interface{}{
			"nzbget": map[string]interface{}{
				"summary": summary,
			},
		},
		Details: map[string]interface{}{
			"nzbget": map[string]interface{}{
				"queueCount":       queueCount,
				"failedCount":      summary.FailedCount,
				"downloadPaused":   paused,
				"quotaReached":     summary.Status.QuotaReached,
				"speedBps":         downloadRate,
				"remainingBytes":   remainingBytes,
				"freeDiskBytes":    freeBytes,
				"recentFailureLen": len(summary.RecentFailures),
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

func parseSummaryCount(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if intVal, err := strconv.Atoi(raw); err == nil {
		return intVal
	}
	floatVal, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int(floatVal)
}

func joinHiLo(hi, lo uint64) uint64 {
	return (hi << 32) | (lo & 0xffffffff)
}
