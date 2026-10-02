// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"testing"

	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestCountTranscodingSessions(t *testing.T) {
	t.Parallel()

	sessions := []types.PlexSession{
		{},
		{TranscodeSession: &types.PlexTranscodeSession{}},
		{TranscodeSession: &types.PlexTranscodeSession{}},
	}

	if got := countTranscodingSessions(sessions); got != 2 {
		t.Fatalf("countTranscodingSessions() = %d, want 2", got)
	}
}

func TestCountOnlineDevices(t *testing.T) {
	t.Parallel()

	devices := []tailscale.Device{
		{Online: true},
		{Online: false},
		{Online: true},
	}

	if got := countOnlineDevices(devices); got != 2 {
		t.Fatalf("countOnlineDevices() = %d, want 2", got)
	}
}

func TestCountJellyfinTranscoding(t *testing.T) {
	t.Parallel()

	sessions := []types.JellyfinSession{
		{},
		{TranscodingInfo: &types.JellyfinTranscodingInfo{}},
		{TranscodingInfo: &types.JellyfinTranscodingInfo{}},
	}

	if got := countJellyfinTranscoding(sessions); got != 2 {
		t.Fatalf("countJellyfinTranscoding() = %d, want 2", got)
	}
}

func TestCountJellyfinPaused(t *testing.T) {
	t.Parallel()

	sessions := []types.JellyfinSession{
		{},
		{PlayState: &types.JellyfinPlayerState{IsPaused: true}},
		{PlayState: &types.JellyfinPlayerState{IsPaused: false}},
	}

	if got := countJellyfinPaused(sessions); got != 1 {
		t.Fatalf("countJellyfinPaused() = %d, want 1", got)
	}
}

func TestCountUptimeKumaStates(t *testing.T) {
	t.Parallel()

	monitors := []types.UptimeKumaMonitor{
		{Status: "up"},
		{Status: "up"},
		{Status: "down"},
		{Status: "pending"},
		{Status: "maintenance"},
	}

	total, up, down, pending, maintenance := countUptimeKumaStates(monitors)
	if total != 5 || up != 2 || down != 1 || pending != 1 || maintenance != 1 {
		t.Fatalf(
			"countUptimeKumaStates() = total:%d up:%d down:%d pending:%d maintenance:%d, want total:5 up:2 down:1 pending:1 maintenance:1",
			total, up, down, pending, maintenance,
		)
	}
}

func TestSummarizeQuiCardStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary types.QuiTransferSummary
		want    string
	}{
		{
			name:    "no instances",
			summary: types.QuiTransferSummary{},
			want:    "warning",
		},
		{
			name: "no active instances",
			summary: types.QuiTransferSummary{
				TotalInstances: 2,
			},
			want: "warning",
		},
		{
			name: "partial connectivity",
			summary: types.QuiTransferSummary{
				TotalInstances:     3,
				ActiveInstances:    2,
				ConnectedInstances: 1,
			},
			want: "warning",
		},
		{
			name: "all active connected",
			summary: types.QuiTransferSummary{
				TotalInstances:     2,
				ActiveInstances:    2,
				ConnectedInstances: 2,
			},
			want: "online",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := summarizeQuiCardStatus(tt.summary); got != tt.want {
				t.Fatalf("summarizeQuiCardStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
