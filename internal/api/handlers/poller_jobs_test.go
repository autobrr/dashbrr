// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import "testing"

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
