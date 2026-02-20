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

func TestSummarizeRadarrQueue(t *testing.T) {
	t.Parallel()

	records := []types.RadarrQueueRecord{
		{Status: "downloading", Size: 1024},
		{Status: "completed", Size: 2048},
		{Status: "downloading", Size: 512},
	}

	downloading, totalSize := summarizeRadarrQueue(records)
	if downloading != 2 {
		t.Fatalf("summarizeRadarrQueue() downloading = %d, want 2", downloading)
	}
	if totalSize != 3584 {
		t.Fatalf("summarizeRadarrQueue() totalSize = %d, want 3584", totalSize)
	}
}

func TestSummarizeLidarrQueue(t *testing.T) {
	t.Parallel()

	records := []types.LidarrQueueItem{
		{Status: "downloading", Size: 256},
		{Status: "completed", Size: 512},
		{Status: "downloading", Size: 128},
	}

	downloading, totalSize := summarizeLidarrQueue(records)
	if downloading != 2 {
		t.Fatalf("summarizeLidarrQueue() downloading = %d, want 2", downloading)
	}
	if totalSize != 896 {
		t.Fatalf("summarizeLidarrQueue() totalSize = %d, want 896", totalSize)
	}
}

func TestSummarizeReadarrQueue(t *testing.T) {
	t.Parallel()

	records := []types.ReadarrQueueItem{
		{Status: "downloading", Size: 64},
		{Status: "completed", Size: 128},
		{Status: "downloading", Size: 32},
	}

	downloading, totalSize := summarizeReadarrQueue(records)
	if downloading != 2 {
		t.Fatalf("summarizeReadarrQueue() downloading = %d, want 2", downloading)
	}
	if totalSize != 224 {
		t.Fatalf("summarizeReadarrQueue() totalSize = %d, want 224", totalSize)
	}
}

func TestSummarizeSonarrQueue(t *testing.T) {
	t.Parallel()

	records := []types.QueueRecord{
		{Status: "downloading", Size: 100, Episodes: []types.EpisodeBasic{{}, {}}},
		{Status: "queued", Size: 250, Episodes: []types.EpisodeBasic{{}}},
		{Status: "downloading", Size: 50, Episodes: nil},
	}

	downloading, episodeCount, totalSize := summarizeSonarrQueue(records)
	if downloading != 2 {
		t.Fatalf("summarizeSonarrQueue() downloading = %d, want 2", downloading)
	}
	if episodeCount != 3 {
		t.Fatalf("summarizeSonarrQueue() episodeCount = %d, want 3", episodeCount)
	}
	if totalSize != 400 {
		t.Fatalf("summarizeSonarrQueue() totalSize = %d, want 400", totalSize)
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := summarizeQuiCardStatus(tt.summary); got != tt.want {
				t.Fatalf("summarizeQuiCardStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
