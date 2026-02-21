// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

func decodeSingleSnapshotHealth(t *testing.T, bc *Broadcaster) models.ServiceHealth {
	t.Helper()

	snapshot := bc.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 snapshot payload, got %d", len(snapshot))
	}

	var decoded models.ServiceHealth
	raw := strings.TrimPrefix(string(snapshot[0]), "data: ")
	raw = strings.TrimSuffix(raw, "\n\n")
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode snapshot payload: %v", err)
	}

	return decoded
}

func TestBroadcasterSnapshotKeepsLatestPerService(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	first := models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "first",
		LastChecked: now,
	}
	second := models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "warning",
		Message:     "second",
		LastChecked: now.Add(time.Second),
	}

	bc.Publish(first)
	bc.Publish(second)

	snapshot := bc.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 snapshot payload, got %d", len(snapshot))
	}

	want := string(EncodeHealthAsSSE(normalizeServiceEvent(second)))
	got := string(snapshot[0])
	if got != want {
		t.Fatalf("unexpected payload\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBroadcasterPublishNormalizesImplicitInternalEventType(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())

	bc.Publish(models.ServiceHealth{
		ServiceID: "radarr-1",
		Status:    "online",
		Message:   "radarr_queue",
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.EventType != models.ServiceEventInternal {
		t.Fatalf("snapshot eventType = %q, want %q", decoded.EventType, models.ServiceEventInternal)
	}
	if decoded.LastChecked.IsZero() {
		t.Fatalf("snapshot lastChecked should be set")
	}
}

func TestBroadcasterPublishNormalizesImplicitHealthEventType(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())

	bc.Publish(models.ServiceHealth{
		ServiceID: "radarr-1",
		Status:    "online",
		Message:   "Healthy",
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.EventType != models.ServiceEventHealth {
		t.Fatalf("snapshot eventType = %q, want %q", decoded.EventType, models.ServiceEventHealth)
	}
	if decoded.LastChecked.IsZero() {
		t.Fatalf("snapshot lastChecked should be set")
	}
}

func TestBroadcasterSnapshotPreservesVersionAcrossPartialUpdates(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "Healthy",
		LastChecked: now,
		Version:     "6.1.1",
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "radarr_queue",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"radarr": map[string]any{
				"queue": map[string]any{"totalRecords": float64(2)},
			},
		},
	})

	snapshot := bc.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 snapshot payload, got %d", len(snapshot))
	}

	var event struct {
		Data models.ServiceHealth `json:"-"`
	}
	raw := strings.TrimPrefix(string(snapshot[0]), "data: ")
	raw = strings.TrimSuffix(raw, "\n\n")
	if err := json.Unmarshal([]byte(raw), &event.Data); err != nil {
		t.Fatalf("failed to decode snapshot payload: %v", err)
	}

	if event.Data.Version != "6.1.1" {
		t.Fatalf("snapshot version = %q, want %q", event.Data.Version, "6.1.1")
	}
	if event.Data.Message != "Healthy" {
		t.Fatalf("snapshot message = %q, want %q", event.Data.Message, "Healthy")
	}
}

func TestBroadcasterSnapshotKeepsWarningStateAcrossInternalEvents(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "warning",
		Message:     "[IndexerLongTermStatusCheck] Indexers unavailable",
		LastChecked: now,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "online",
		Message:     "prowlarr_indexers",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"prowlarr": map[string]any{
				"indexers": []any{"a"},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.Status != "warning" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "warning")
	}
	if decoded.Message != "[IndexerLongTermStatusCheck] Indexers unavailable" {
		t.Fatalf("snapshot message = %q", decoded.Message)
	}
}

func TestBroadcasterSnapshotKeepsHealthResponseTimeAcrossInternalEvents(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:       "radarr-1",
		Status:          "online",
		Message:         "Healthy",
		LastChecked:     now,
		ResponseTime:    42,
		UpdateAvailable: true,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:       "radarr-1",
		Status:          "online",
		Message:         "radarr_queue",
		LastChecked:     now.Add(time.Second),
		ResponseTime:    0,
		UpdateAvailable: false,
		Stats: map[string]any{
			"radarr": map[string]any{
				"queue": map[string]any{"totalRecords": float64(2)},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.ResponseTime != 42 {
		t.Fatalf("snapshot responseTime = %d, want %d", decoded.ResponseTime, 42)
	}
	if !decoded.UpdateAvailable {
		t.Fatalf("snapshot updateAvailable = %v, want true", decoded.UpdateAvailable)
	}
	if decoded.EventType != models.ServiceEventHealth {
		t.Fatalf("snapshot eventType = %q, want %q", decoded.EventType, models.ServiceEventHealth)
	}
}

func TestBroadcasterSnapshotKeepsWarningVersionAndResponseTimeAcrossInternalEvents(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	publishHealthServiceUpdate(bc, models.ServiceHealth{
		ServiceID:    "prowlarr-1",
		Status:       "warning",
		Message:      "[IndexerLongTermStatusCheck] Indexers unavailable",
		LastChecked:  now,
		Version:      "2.3.2",
		ResponseTime: 11,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "online",
		Message:     "prowlarr_indexers",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"prowlarr": map[string]any{
				"indexers": []any{"a"},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.Status != "warning" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "warning")
	}
	if decoded.Message != "[IndexerLongTermStatusCheck] Indexers unavailable" {
		t.Fatalf("snapshot message = %q", decoded.Message)
	}
	if decoded.Version != "2.3.2" {
		t.Fatalf("snapshot version = %q, want %q", decoded.Version, "2.3.2")
	}
	if decoded.ResponseTime != 11 {
		t.Fatalf("snapshot responseTime = %d, want %d", decoded.ResponseTime, 11)
	}
}

func TestBroadcasterSnapshotPromotesInternalEventToHealthEventType(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "unknown",
		Message:     "prowlarr_indexers",
		EventType:   models.ServiceEventInternal,
		LastChecked: now.Add(-time.Second),
	})
	publishHealthServiceUpdate(bc, models.ServiceHealth{
		ServiceID:    "prowlarr-1",
		Status:       "warning",
		Message:      "[IndexerLongTermStatusCheck] Indexers unavailable",
		LastChecked:  now,
		ResponseTime: 7,
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.EventType != models.ServiceEventHealth {
		t.Fatalf("snapshot eventType = %q, want %q", decoded.EventType, models.ServiceEventHealth)
	}
	if decoded.Status != "warning" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "warning")
	}
	if decoded.ResponseTime != 7 {
		t.Fatalf("snapshot responseTime = %d, want %d", decoded.ResponseTime, 7)
	}
}

func TestBroadcasterSnapshotAllowsEmptyHealthMessageToClearPriorWarning(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "general-1",
		Status:      "warning",
		Message:     "Old warning",
		LastChecked: now,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "general-1",
		Status:      "online",
		Message:     "",
		EventType:   models.ServiceEventHealth,
		LastChecked: now.Add(time.Second),
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.Status != "online" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "online")
	}
	if decoded.Message != "" {
		t.Fatalf("snapshot message = %q, want empty", decoded.Message)
	}
}

func TestBroadcasterSnapshotTreatsExplicitInternalEventTypeAsInternal(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "sonarr-1",
		Status:      "warning",
		Message:     "Indexer warning",
		LastChecked: now,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "sonarr-1",
		Status:      "online",
		Message:     "Queue refresh completed",
		EventType:   models.ServiceEventInternal,
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"sonarr": map[string]any{"queue": map[string]any{"totalRecords": float64(2)}},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.Status != "warning" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "warning")
	}
	if decoded.Message != "Indexer warning" {
		t.Fatalf("snapshot message = %q, want %q", decoded.Message, "Indexer warning")
	}
}

func TestBroadcasterSnapshotTreatsExplicitHealthEventTypeAsHealth(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "warning",
		Message:     "Old warning",
		LastChecked: now,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "radarr_queue",
		EventType:   models.ServiceEventHealth,
		LastChecked: now.Add(time.Second),
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.Status != "online" {
		t.Fatalf("snapshot status = %q, want %q", decoded.Status, "online")
	}
	if decoded.Message != "radarr_queue" {
		t.Fatalf("snapshot message = %q, want %q", decoded.Message, "radarr_queue")
	}
}

func TestBroadcasterSnapshotClearsUpdateAvailableOnHealthUpdate(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:       "sonarr-1",
		Status:          "online",
		Message:         "Healthy",
		LastChecked:     now,
		UpdateAvailable: true,
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:       "sonarr-1",
		Status:          "online",
		Message:         "Healthy",
		LastChecked:     now.Add(time.Second),
		UpdateAvailable: false,
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	if decoded.UpdateAvailable {
		t.Fatalf("snapshot updateAvailable = %v, want false", decoded.UpdateAvailable)
	}
}

func TestBroadcasterSnapshotMergesNestedStatsPayloads(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "online",
		Message:     "prowlarr_stats",
		LastChecked: now,
		Stats: map[string]any{
			"prowlarr": map[string]any{
				"stats": map[string]any{"grabCount": float64(12)},
			},
		},
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "prowlarr-1",
		Status:      "online",
		Message:     "prowlarr_indexers",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"prowlarr": map[string]any{
				"indexers": []any{"a", "b"},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	prowlarrStats, ok := decoded.Stats["prowlarr"].(map[string]any)
	if !ok {
		t.Fatalf("expected prowlarr stats map, got %T", decoded.Stats["prowlarr"])
	}
	if _, ok := prowlarrStats["stats"]; !ok {
		t.Fatalf("expected merged stats payload to include stats field")
	}
	if _, ok := prowlarrStats["indexers"]; !ok {
		t.Fatalf("expected merged stats payload to include indexers field")
	}
}

func TestBroadcasterSnapshotKeepsAutobrrStatsAndReleases(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "autobrr-1",
		Status:      "online",
		Message:     "autobrr_releases",
		LastChecked: now,
		Stats: map[string]any{
			"autobrr": map[string]any{
				"releases": map[string]any{
					"data":        []any{"r1"},
					"count":       float64(1),
					"next_cursor": float64(2),
				},
			},
		},
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "autobrr-1",
		Status:      "online",
		Message:     "autobrr_stats",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"autobrr": map[string]any{
				"stats": map[string]any{
					"total_count": float64(42),
				},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	autobrrStats, ok := decoded.Stats["autobrr"].(map[string]any)
	if !ok {
		t.Fatalf("expected autobrr stats map, got %T", decoded.Stats["autobrr"])
	}
	if _, ok := autobrrStats["stats"]; !ok {
		t.Fatalf("expected merged autobrr payload to include stats field")
	}
	if _, ok := autobrrStats["releases"]; !ok {
		t.Fatalf("expected merged autobrr payload to include releases field")
	}
}

func TestBroadcasterSnapshotKeepsAutobrrReleasesAfterHealthUpdate(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "autobrr-1",
		Status:      "online",
		Message:     "autobrr_releases",
		LastChecked: now,
		Stats: map[string]any{
			"autobrr": map[string]any{
				"releases": map[string]any{
					"data":        []any{"r1"},
					"count":       float64(1),
					"next_cursor": float64(2),
				},
			},
		},
	})

	bc.Publish(models.ServiceHealth{
		ServiceID:   "autobrr-1",
		Status:      "online",
		Message:     "Autobrr is running",
		LastChecked: now.Add(time.Second),
		Stats: map[string]any{
			"autobrr": map[string]any{
				"stats": map[string]any{
					"total_count": float64(42),
				},
			},
		},
	})

	decoded := decodeSingleSnapshotHealth(t, bc)

	autobrrStats, ok := decoded.Stats["autobrr"].(map[string]any)
	if !ok {
		t.Fatalf("expected autobrr stats map, got %T", decoded.Stats["autobrr"])
	}
	if _, ok := autobrrStats["stats"]; !ok {
		t.Fatalf("expected merged autobrr payload to include stats field")
	}
	if _, ok := autobrrStats["releases"]; !ok {
		t.Fatalf("expected merged autobrr payload to include releases field")
	}
}

func TestBroadcasterSnapshotSortedByServiceID(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "z-service",
		Status:      "online",
		Message:     "z",
		LastChecked: now,
	})
	bc.Publish(models.ServiceHealth{
		ServiceID:   "a-service",
		Status:      "online",
		Message:     "a",
		LastChecked: now,
	})

	snapshot := bc.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 snapshot payloads, got %d", len(snapshot))
	}

	if !strings.Contains(string(snapshot[0]), `"serviceId":"a-service"`) {
		t.Fatalf("expected first payload to be a-service, got %q", string(snapshot[0]))
	}
	if !strings.Contains(string(snapshot[1]), `"serviceId":"z-service"`) {
		t.Fatalf("expected second payload to be z-service, got %q", string(snapshot[1]))
	}
}

func TestBroadcasterSnapshotSkipsEmptyServiceID(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	now := time.Unix(1700000000, 0)

	bc.Publish(models.ServiceHealth{
		Status:      "online",
		Message:     "no-id",
		LastChecked: now,
	})

	snapshot := bc.Snapshot()
	if len(snapshot) != 0 {
		t.Fatalf("expected empty snapshot, got %d", len(snapshot))
	}
}

func TestBroadcasterPublishLatestRepublishesCachedPayload(t *testing.T) {
	hub := sse.NewHub()
	bc := NewBroadcaster(hub)
	now := time.Unix(1700000000, 0)

	ctx := t.Context()

	sub, _ := hub.Subscribe(ctx, 4)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "Healthy",
		LastChecked: now,
	})

	// Drain initial publish.
	select {
	case <-sub:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected initial publish payload")
	}

	if !bc.PublishLatest("radarr-1") {
		t.Fatalf("expected PublishLatest to succeed for known service")
	}

	select {
	case payload := <-sub:
		if !strings.Contains(string(payload), `"serviceId":"radarr-1"`) {
			t.Fatalf("unexpected republished payload: %q", string(payload))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected republished cached payload")
	}
}

func TestBroadcasterPublishLatestUnknownService(t *testing.T) {
	bc := NewBroadcaster(sse.NewHub())
	if bc.PublishLatest("missing-service") {
		t.Fatalf("expected PublishLatest to return false for unknown service")
	}
}
