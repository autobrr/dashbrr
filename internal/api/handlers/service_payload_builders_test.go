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
