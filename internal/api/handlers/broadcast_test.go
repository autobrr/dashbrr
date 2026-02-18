// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

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

	want := string(EncodeHealthAsSSE(second))
	got := string(snapshot[0])
	if got != want {
		t.Fatalf("unexpected payload\nwant: %q\ngot:  %q", want, got)
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

