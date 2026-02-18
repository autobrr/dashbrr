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

func TestCountOnlineTailscaleDevices(t *testing.T) {
	t.Parallel()

	devices := []tailscale.Device{
		{Online: true},
		{Online: false},
		{Online: true},
	}

	if got := countOnlineTailscaleDevices(devices); got != 2 {
		t.Fatalf("countOnlineTailscaleDevices() = %d, want 2", got)
	}
}
