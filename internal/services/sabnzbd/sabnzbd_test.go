// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sabnzbd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetQueue_UsesExpectedQueryParams(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Fatalf("path = %s, want /api", r.URL.Path)
		}
		if got := r.URL.Query().Get("mode"); got != "queue" {
			t.Fatalf("mode = %q, want queue", got)
		}
		if got := r.URL.Query().Get("output"); got != "json" {
			t.Fatalf("output = %q, want json", got)
		}
		if got := r.URL.Query().Get("apikey"); got != "abc123" {
			t.Fatalf("apikey = %q, want abc123", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"queue": {
				"status": "Downloading",
				"speed": "12.34 MB",
				"noofslots": "1",
				"have_warnings": "0",
				"slots": [
					{
						"nzo_id": "SABnzbd_nzo_123",
						"filename": "Example.Release",
						"status": "Downloading",
						"sizeleft": "1.2 GB",
						"timeleft": "0:10:00",
						"percentage": "50"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	service := &SabnzbdService{}
	queue, err := service.GetQueue(context.Background(), server.URL, "abc123")
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}

	if queue.Status != "Downloading" {
		t.Fatalf("status = %q, want Downloading", queue.Status)
	}
	if len(queue.Slots) != 1 {
		t.Fatalf("slots len = %d, want 1", len(queue.Slots))
	}
	if queue.Slots[0].NzoID != "SABnzbd_nzo_123" {
		t.Fatalf("nzo_id = %q, want SABnzbd_nzo_123", queue.Slots[0].NzoID)
	}
}

func TestGetSummary_CombinesQueueAndFailedHistory(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		w.Header().Set("Content-Type", "application/json")

		switch mode {
		case "queue":
			_, _ = w.Write([]byte(`{
				"queue": {
					"status": "Idle",
					"speed": "0.00 B",
					"noofslots": "0",
					"have_warnings": "1",
					"slots": []
				}
			}`))
		case "history":
			_, _ = w.Write([]byte(`{
				"history": {
					"noofslots": 5,
					"slots": [
						{
							"nzo_id": "SABnzbd_nzo_fail_1",
							"name": "Broken.Release",
							"status": "Failed",
							"fail_message": "Missing articles"
						},
						{
							"nzo_id": "SABnzbd_nzo_fail_2",
							"name": "Another.Broken.Release",
							"status": "Failed",
							"fail_message": "Repair failed"
						}
					]
				}
			}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	service := &SabnzbdService{}
	summary, err := service.GetSummary(context.Background(), server.URL, "abc123")
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}

	if summary.Queue.HaveWarnings != "1" {
		t.Fatalf("have_warnings = %q, want 1", summary.Queue.HaveWarnings)
	}
	if summary.FailedCount != 5 {
		t.Fatalf("failedCount = %d, want 5", summary.FailedCount)
	}
	if len(summary.RecentFailures) != 2 {
		t.Fatalf("recentFailures len = %d, want 2", len(summary.RecentFailures))
	}
}

// TestGetQueue_NumericCountFields exercises the Sabnzbd >=4.5.5 response
// format where noofslots and noofslots_total are bare JSON numbers instead
// of quoted strings (issue #90).
func TestGetQueue_NumericCountFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"queue": {
				"status": "Downloading",
				"speed": "5.00 MB",
				"noofslots": 3,
				"noofslots_total": 10,
				"have_warnings": 0,
				"slots": []
			}
		}`))
	}))
	defer server.Close()

	service := &SabnzbdService{}
	queue, err := service.GetQueue(context.Background(), server.URL, "key")
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}
	if queue.NoOfSlots != "3" {
		t.Fatalf("noofslots = %q, want 3", queue.NoOfSlots)
	}
	if queue.NoOfSlotsTotal != "10" {
		t.Fatalf("noofslots_total = %q, want 10", queue.NoOfSlotsTotal)
	}
	if queue.HaveWarnings != "0" {
		t.Fatalf("have_warnings = %q, want 0", queue.HaveWarnings)
	}
}

func TestCheckHealth_WarnsWhenPausedWithWarnings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		w.Header().Set("Content-Type", "application/json")

		switch mode {
		case "version":
			_, _ = w.Write([]byte(`{"version":"4.5.3"}`))
		case "queue":
			_, _ = w.Write([]byte(`{
				"queue": {
					"status": "Paused",
					"have_warnings": "2",
					"diskspace1": "1.0",
					"diskspacetotal1": "100.0",
					"diskspace2": "2.0",
					"diskspacetotal2": "100.0",
					"slots": []
				}
			}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	service := &SabnzbdService{}
	health, statusCode := service.CheckHealth(context.Background(), server.URL, "abc123")
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "warning" {
		t.Fatalf("health.Status = %q, want warning", health.Status)
	}
	if !strings.Contains(health.Message, "Paused") {
		t.Fatalf("message = %q, want paused warning", health.Message)
	}
	if !strings.Contains(health.Message, "warning(s)") {
		t.Fatalf("message = %q, want warning count", health.Message)
	}
	if !strings.Contains(health.Message, "Low complete disk space") {
		t.Fatalf("message = %q, want low disk warning", health.Message)
	}
}
