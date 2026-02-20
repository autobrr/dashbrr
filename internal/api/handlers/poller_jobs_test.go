// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"testing"
	"time"
)

func TestNewPoller_AutobrrJobsAreSplit(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["autobrr"]

	if len(jobs) != 3 {
		t.Fatalf("autobrr job count = %d, want 3", len(jobs))
	}

	want := []string{"autobrr_stats", "autobrr_irc_status", "autobrr_releases"}
	for i, name := range want {
		if jobs[i].name != name {
			t.Fatalf("autobrr job[%d] = %q, want %q", i, jobs[i].name, name)
		}
	}
}

func TestNewPoller_ProwlarrJobsAreSplit(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["prowlarr"]

	if len(jobs) != 2 {
		t.Fatalf("prowlarr job count = %d, want 2", len(jobs))
	}

	want := []string{"prowlarr_stats", "prowlarr_indexers"}
	for i, name := range want {
		if jobs[i].name != name {
			t.Fatalf("prowlarr job[%d] = %q, want %q", i, jobs[i].name, name)
		}
	}
}

func TestNewPoller_QuiJobsAreOverviewOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["qui"]

	if len(jobs) != 1 {
		t.Fatalf("qui job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "qui_overview" {
		t.Fatalf("qui job[0] = %q, want %q", jobs[0].name, "qui_overview")
	}
}

func TestNewPoller_LidarrJobsAreQueueOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["lidarr"]

	if len(jobs) != 1 {
		t.Fatalf("lidarr job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "lidarr_queue" {
		t.Fatalf("lidarr job[0] = %q, want %q", jobs[0].name, "lidarr_queue")
	}
}

func TestNewPoller_ReadarrJobsAreQueueOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["readarr"]

	if len(jobs) != 1 {
		t.Fatalf("readarr job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "readarr_queue" {
		t.Fatalf("readarr job[0] = %q, want %q", jobs[0].name, "readarr_queue")
	}
}

func TestNewPoller_BazarrJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["bazarr"]

	if len(jobs) != 1 {
		t.Fatalf("bazarr job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "bazarr_summary" {
		t.Fatalf("bazarr job[0] = %q, want %q", jobs[0].name, "bazarr_summary")
	}
}

func TestEffectiveJobTimeout(t *testing.T) {
	t.Parallel()

	if got := effectiveJobTimeout(0); got != pollerDefaultJobTimeout {
		t.Fatalf("effectiveJobTimeout(0) = %v, want %v", got, pollerDefaultJobTimeout)
	}

	override := 7 * time.Second
	if got := effectiveJobTimeout(override); got != override {
		t.Fatalf("effectiveJobTimeout(override) = %v, want %v", got, override)
	}
}

func TestPollerStaleDataThreshold(t *testing.T) {
	t.Parallel()

	if got := pollerStaleDataThreshold(5 * time.Second); got != pollerMinStaleThreshold {
		t.Fatalf("pollerStaleDataThreshold(5s) = %v, want %v", got, pollerMinStaleThreshold)
	}

	if got := pollerStaleDataThreshold(40 * time.Second); got != 80*time.Second {
		t.Fatalf("pollerStaleDataThreshold(40s) = %v, want %v", got, 80*time.Second)
	}

	if got := pollerStaleDataThreshold(8 * time.Minute); got != pollerMaxStaleThreshold {
		t.Fatalf("pollerStaleDataThreshold(8m) = %v, want %v", got, pollerMaxStaleThreshold)
	}
}

func TestApplyPollerJobJitter(t *testing.T) {
	t.Parallel()

	base := 60 * time.Second
	a := applyPollerJobJitter("sonarr-1:sonarr_queue", base)
	b := applyPollerJobJitter("sonarr-1:sonarr_queue", base)

	if a != b {
		t.Fatalf("jitter should be deterministic, got %v and %v", a, b)
	}
	if a < base {
		t.Fatalf("jittered interval should not be below base: got %v base %v", a, base)
	}
	if a > base+pollerMaxJobJitter {
		t.Fatalf("jittered interval should be bounded: got %v max %v", a, base+pollerMaxJobJitter)
	}
}
