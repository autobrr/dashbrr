package handlers

import (
	"testing"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestBuildAutobrrIRCServiceUpdate_Healthy(t *testing.T) {
	health, eventType := buildAutobrrIRCServiceUpdate("autobrr-1", []types.IRCStatus{
		{Name: "main", Healthy: true, Enabled: true},
	})

	if eventType != models.ServiceEventInternal {
		t.Fatalf("eventType = %q, want %q", eventType, models.ServiceEventInternal)
	}
	if health.Status != "online" {
		t.Fatalf("status = %q, want online", health.Status)
	}
	if health.Message != "autobrr_irc_status" {
		t.Fatalf("message = %q, want autobrr_irc_status", health.Message)
	}
}

func TestBuildJellyfinSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.JellyfinSummaryResponse{
		System: types.JellyfinSystemInfo{
			ServerName: "Jellyfin",
			Version:    "10.10.7",
		},
		Sessions: []types.JellyfinSession{
			{
				ID:       "1",
				UserName: "alice",
				PlayState: &types.JellyfinPlayerState{
					IsPaused: false,
				},
				NowPlayingItem:  &types.JellyfinNowPlayingItem{Name: "Movie A"},
				TranscodingInfo: &types.JellyfinTranscodingInfo{VideoCodec: "h264"},
			},
			{
				ID:       "2",
				UserName: "bob",
				PlayState: &types.JellyfinPlayerState{
					IsPaused: true,
				},
				NowPlayingItem: &types.JellyfinNowPlayingItem{Name: "Movie B"},
			},
		},
	}

	health := buildJellyfinSummaryServiceUpdate("jellyfin-1", summary)
	if health.Message != "jellyfin_summary" {
		t.Fatalf("message = %q, want jellyfin_summary", health.Message)
	}

	stats, ok := health.Stats["jellyfin"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected jellyfin stats object")
	}
	gotSummary, ok := stats["summary"].(*types.JellyfinSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if gotSummary.System.Version != "10.10.7" {
		t.Fatalf("version = %q, want 10.10.7", gotSummary.System.Version)
	}

	details, ok := health.Details["jellyfin"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected jellyfin details object")
	}
	if got := details["activeStreams"]; got != 2 {
		t.Fatalf("activeStreams = %v, want 2", got)
	}
	if got := details["transcoding"]; got != 1 {
		t.Fatalf("transcoding = %v, want 1", got)
	}
	if got := details["paused"]; got != 1 {
		t.Fatalf("paused = %v, want 1", got)
	}
}

func TestBuildUptimeKumaSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.UptimeKumaSummaryResponse{
		Monitors: []types.UptimeKumaMonitor{
			{ID: "1", Name: "API", Status: "up", ResponseTimeMs: 32},
			{ID: "2", Name: "DB", Status: "down"},
			{ID: "3", Name: "Queue", Status: "pending"},
		},
	}

	health := buildUptimeKumaSummaryServiceUpdate("uptimekuma-1", summary)
	if health.Message != "uptimekuma_summary" {
		t.Fatalf("message = %q, want uptimekuma_summary", health.Message)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}

	stats, ok := health.Stats["uptimekuma"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected uptimekuma stats object")
	}
	gotSummary, ok := stats["summary"].(*types.UptimeKumaSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if len(gotSummary.Monitors) != 3 {
		t.Fatalf("summary monitor count = %d, want 3", len(gotSummary.Monitors))
	}

	details, ok := health.Details["uptimekuma"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected uptimekuma details object")
	}
	if got := details["total"]; got != 3 {
		t.Fatalf("total = %v, want 3", got)
	}
	if got := details["up"]; got != 1 {
		t.Fatalf("up = %v, want 1", got)
	}
	if got := details["down"]; got != 1 {
		t.Fatalf("down = %v, want 1", got)
	}
	if got := details["pending"]; got != 1 {
		t.Fatalf("pending = %v, want 1", got)
	}
	if got := details["issues"]; got != 2 {
		t.Fatalf("issues = %v, want 2", got)
	}
}

func TestBuildTraefikSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.TraefikSummaryResponse{
		Overview: types.TraefikOverviewResponse{
			HTTP: types.TraefikSchemeOverview{
				Routers:     &types.TraefikSection{Total: 10, Warnings: 2, Errors: 1},
				Services:    &types.TraefikSection{Total: 8, Warnings: 1, Errors: 0},
				Middlewares: &types.TraefikSection{Total: 6, Warnings: 0, Errors: 1},
			},
			TCP: types.TraefikSchemeOverview{
				Routers:     &types.TraefikSection{Total: 2, Warnings: 0, Errors: 0},
				Services:    &types.TraefikSection{Total: 2, Warnings: 0, Errors: 0},
				Middlewares: &types.TraefikSection{Total: 1, Warnings: 0, Errors: 0},
			},
			Features:  types.TraefikFeatures{Metrics: "Prometheus", Tracing: "jaeger", AccessLog: true},
			Providers: []string{"Docker", "File"},
		},
		IssueRouters: []types.TraefikRouter{
			{Name: "warn@docker", Status: "warning"},
			{Name: "down@docker", Status: "disabled"},
		},
	}

	health := buildTraefikSummaryServiceUpdate("traefik-1", summary)
	if health.Message != "traefik_summary" {
		t.Fatalf("message = %q, want traefik_summary", health.Message)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}

	stats, ok := health.Stats["traefik"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected traefik stats object")
	}
	gotSummary, ok := stats["summary"].(*types.TraefikSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if len(gotSummary.IssueRouters) != 2 {
		t.Fatalf("issue routers = %d, want 2", len(gotSummary.IssueRouters))
	}

	details, ok := health.Details["traefik"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected traefik details object")
	}
	if got := details["routerTotal"]; got != 12 {
		t.Fatalf("routerTotal = %v, want 12", got)
	}
	if got := details["routerWarnings"]; got != 2 {
		t.Fatalf("routerWarnings = %v, want 2", got)
	}
	if got := details["routerErrors"]; got != 1 {
		t.Fatalf("routerErrors = %v, want 1", got)
	}
	if got := details["providers"]; got != 2 {
		t.Fatalf("providers = %v, want 2", got)
	}
	if got := details["issueRouters"]; got != 2 {
		t.Fatalf("issueRouters = %v, want 2", got)
	}
}

func TestBuildAutobrrIRCServiceUpdate_Unhealthy(t *testing.T) {
	health, eventType := buildAutobrrIRCServiceUpdate("autobrr-1", []types.IRCStatus{
		{Name: "alpha", Healthy: false, Enabled: true},
	})

	if eventType != models.ServiceEventHealth {
		t.Fatalf("eventType = %q, want %q", eventType, models.ServiceEventHealth)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}
	if health.Message != "IRC network alpha is unhealthy" {
		t.Fatalf("message = %q, want IRC warning", health.Message)
	}
}

func TestBuildRadarrQueueServiceUpdate_DetailsAndStats(t *testing.T) {
	resp := &types.RadarrQueueResponse{
		TotalRecords: 2,
		Records: []types.RadarrQueueRecord{
			{Status: "downloading", Size: 100},
			{Status: "queued", Size: 50},
		},
	}

	health := buildRadarrQueueServiceUpdate("radarr-1", resp)

	if health.Message != "radarr_queue" {
		t.Fatalf("message = %q, want radarr_queue", health.Message)
	}

	stats, ok := health.Stats["radarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected radarr stats object")
	}
	queue, ok := stats["queue"].(*types.RadarrQueueResponse)
	if !ok {
		t.Fatalf("expected queue payload pointer")
	}
	if queue.TotalRecords != 2 {
		t.Fatalf("queue.TotalRecords = %d, want 2", queue.TotalRecords)
	}

	details, ok := health.Details["radarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected radarr details object")
	}
	if got := details["downloadingCount"]; got != 1 {
		t.Fatalf("downloadingCount = %v, want 1", got)
	}
	if got := details["totalSize"]; got != int64(150) {
		t.Fatalf("totalSize = %v, want 150", got)
	}
}

func TestBuildLidarrQueueServiceUpdate_DetailsAndStats(t *testing.T) {
	resp := &types.LidarrQueueResponse{
		TotalRecords: 2,
		Records: []types.LidarrQueueItem{
			{Status: "downloading", Size: 42},
			{Status: "queued", Size: 8},
		},
	}

	health := buildLidarrQueueServiceUpdate("lidarr-1", resp)

	if health.Message != "lidarr_queue" {
		t.Fatalf("message = %q, want lidarr_queue", health.Message)
	}

	stats, ok := health.Stats["lidarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected lidarr stats object")
	}
	queue, ok := stats["queue"].(*types.LidarrQueueResponse)
	if !ok {
		t.Fatalf("expected queue payload pointer")
	}
	if queue.TotalRecords != 2 {
		t.Fatalf("queue.TotalRecords = %d, want 2", queue.TotalRecords)
	}

	details, ok := health.Details["lidarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected lidarr details object")
	}
	if got := details["downloadingCount"]; got != 1 {
		t.Fatalf("downloadingCount = %v, want 1", got)
	}
	if got := details["totalSize"]; got != int64(50) {
		t.Fatalf("totalSize = %v, want 50", got)
	}
}

func TestBuildReadarrQueueServiceUpdate_DetailsAndStats(t *testing.T) {
	resp := &types.ReadarrQueueResponse{
		TotalRecords: 2,
		Records: []types.ReadarrQueueItem{
			{Status: "downloading", Size: 84},
			{Status: "queued", Size: 16},
		},
	}

	health := buildReadarrQueueServiceUpdate("readarr-1", resp)

	if health.Message != "readarr_queue" {
		t.Fatalf("message = %q, want readarr_queue", health.Message)
	}

	stats, ok := health.Stats["readarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected readarr stats object")
	}
	queue, ok := stats["queue"].(*types.ReadarrQueueResponse)
	if !ok {
		t.Fatalf("expected queue payload pointer")
	}
	if queue.TotalRecords != 2 {
		t.Fatalf("queue.TotalRecords = %d, want 2", queue.TotalRecords)
	}

	details, ok := health.Details["readarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected readarr details object")
	}
	if got := details["downloadingCount"]; got != 1 {
		t.Fatalf("downloadingCount = %v, want 1", got)
	}
	if got := details["totalSize"]; got != int64(100) {
		t.Fatalf("totalSize = %v, want 100", got)
	}
}

func TestBuildBazarrSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.BazarrSummaryResponse{
		Badges: types.BazarrBadges{
			Episodes:      5,
			Movies:        3,
			Status:        1,
			SonarrSignalR: "LIVE",
			RadarrSignalR: "",
		},
		Providers: []types.BazarrProviderStatus{
			{Name: "opensubtitles", Status: "Throttle", Retry: "10m"},
		},
		HealthIssues: []types.BazarrHealthIssue{
			{Object: "Series Root", Issue: "Path not accessible"},
			{Object: "Languages", Issue: "Missing profile"},
		},
	}

	health := buildBazarrSummaryServiceUpdate("bazarr-1", summary)
	if health.Message != "bazarr_summary" {
		t.Fatalf("message = %q, want bazarr_summary", health.Message)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}

	stats, ok := health.Stats["bazarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected bazarr stats object")
	}
	gotSummary, ok := stats["summary"].(*types.BazarrSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if gotSummary.Badges.Episodes != 5 {
		t.Fatalf("badges.episodes = %d, want 5", gotSummary.Badges.Episodes)
	}

	details, ok := health.Details["bazarr"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected bazarr details object")
	}
	if got := details["episodeBacklog"]; got != 5 {
		t.Fatalf("episodeBacklog = %v, want 5", got)
	}
	if got := details["providersWithIssues"]; got != 1 {
		t.Fatalf("providersWithIssues = %v, want 1", got)
	}
	if got := details["healthIssues"]; got != 2 {
		t.Fatalf("healthIssues = %v, want 2", got)
	}
}

func TestBuildSabnzbdSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.SabnzbdSummaryResponse{
		Queue: types.SabnzbdQueue{
			Status:         "Downloading",
			Speed:          "12.34 MB",
			TimeLeft:       "0:12:34",
			SizeLeft:       "8.2 GB",
			NoOfSlots:      "3",
			NoOfSlotsTotal: "7",
			HaveWarnings:   "1",
			Diskspace1Norm: "120.0 GB",
			Diskspace2Norm: "980.0 GB",
			Slots:          []types.SabnzbdQueueSlot{{NzoID: "A"}},
		},
		FailedCount:    4,
		RecentFailures: []types.SabnzbdHistorySlot{{NzoID: "F1"}},
	}

	health := buildSabnzbdSummaryServiceUpdate("sabnzbd-1", summary)
	if health.Message != "sabnzbd_summary" {
		t.Fatalf("message = %q, want sabnzbd_summary", health.Message)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}

	stats, ok := health.Stats["sabnzbd"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sabnzbd stats object")
	}
	gotSummary, ok := stats["summary"].(*types.SabnzbdSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if gotSummary.FailedCount != 4 {
		t.Fatalf("failedCount = %d, want 4", gotSummary.FailedCount)
	}

	details, ok := health.Details["sabnzbd"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sabnzbd details object")
	}
	if got := details["queueCount"]; got != 3 {
		t.Fatalf("queueCount = %v, want 3", got)
	}
	if got := details["totalQueueCount"]; got != 7 {
		t.Fatalf("totalQueueCount = %v, want 7", got)
	}
	if got := details["warningsCount"]; got != 1 {
		t.Fatalf("warningsCount = %v, want 1", got)
	}
	if got := details["recentFailureLen"]; got != 1 {
		t.Fatalf("recentFailureLen = %v, want 1", got)
	}
}

func TestBuildNzbgetSummaryServiceUpdate_DetailsAndStats(t *testing.T) {
	summary := &types.NzbgetSummaryResponse{
		Status: types.NzbgetStatus{
			DownloadPaused:  true,
			QuotaReached:    true,
			DownloadRateLo:  1024,
			DownloadRateHi:  0,
			RemainingSizeLo: 2048,
			RemainingSizeHi: 0,
			FreeDiskSpaceLo: 4096,
			FreeDiskSpaceHi: 0,
		},
		Queue: []types.NzbgetQueueItem{
			{NZBID: 1, NZBName: "A", Status: "DOWNLOADING"},
		},
		FailedCount:    3,
		RecentFailures: []types.NzbgetHistoryItem{{NZBID: 10}},
	}

	health := buildNzbgetSummaryServiceUpdate("nzbget-1", summary)
	if health.Message != "nzbget_summary" {
		t.Fatalf("message = %q, want nzbget_summary", health.Message)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}

	stats, ok := health.Stats["nzbget"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nzbget stats object")
	}
	gotSummary, ok := stats["summary"].(*types.NzbgetSummaryResponse)
	if !ok {
		t.Fatalf("expected summary payload pointer")
	}
	if gotSummary.FailedCount != 3 {
		t.Fatalf("failedCount = %d, want 3", gotSummary.FailedCount)
	}

	details, ok := health.Details["nzbget"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nzbget details object")
	}
	if got := details["queueCount"]; got != 1 {
		t.Fatalf("queueCount = %v, want 1", got)
	}
	if got := details["downloadPaused"]; got != true {
		t.Fatalf("downloadPaused = %v, want true", got)
	}
	if got := details["quotaReached"]; got != true {
		t.Fatalf("quotaReached = %v, want true", got)
	}
	if got := details["speedBps"]; got != uint64(1024) {
		t.Fatalf("speedBps = %v, want 1024", got)
	}
	if got := details["recentFailureLen"]; got != 1 {
		t.Fatalf("recentFailureLen = %v, want 1", got)
	}
}
