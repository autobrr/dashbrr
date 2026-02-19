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
