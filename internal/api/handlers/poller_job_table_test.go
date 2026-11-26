// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"fmt"
	"maps"
	"slices"
	"testing"
)

func TestNewPollerJobTable(t *testing.T) {
	want := map[string][]string{
		"autobrr":     {"autobrr_stats 2m0s 20s", "autobrr_irc_status 2m0s 20s", "autobrr_releases 2m0s 35s"},
		"bazarr":      {"bazarr_summary 1m30s 20s"},
		"emby":        {"emby_summary 10s 12s"},
		"jellyfin":    {"jellyfin_summary 10s 12s"},
		"lidarr":      {"lidarr_queue 1m0s 20s"},
		"maintainerr": {"maintainerr_collections 10m0s 35s"},
		"nzbget":      {"nzbget_summary 45s 20s"},
		"overseerr":   {"overseerr_requests 1m0s 20s"},
		"plex":        {"plex_sessions 10s 12s"},
		"prowlarr":    {"prowlarr_stats 2m0s 20s", "prowlarr_indexers 2m0s 20s"},
		"qui":         {"qui_overview 20s 12s"},
		"radarr":      {"radarr_queue 1m0s 20s"},
		"readarr":     {"readarr_queue 1m0s 20s"},
		"sabnzbd":     {"sabnzbd_summary 45s 20s"},
		"sonarr":      {"sonarr_queue 1m0s 20s"},
		"tailscale":   {"tailscale_devices 1m0s 20s"},
		"traefik":     {"traefik_summary 30s 20s"},
		"uptimekuma":  {"uptimekuma_summary 30s 20s"},
		"whisparr":    {"whisparr_queue 1m0s 20s"},
	}

	got := map[string][]string{}
	for serviceType, jobs := range NewPoller(nil, nil).jobs {
		if len(jobs) == 0 {
			continue
		}
		for _, job := range jobs {
			if job.run == nil {
				t.Errorf("%s job %s has no runner", serviceType, job.name)
			}
			got[serviceType] = append(got[serviceType], fmt.Sprintf("%s %s %s", job.name, job.interval, job.timeout))
		}
	}

	for _, serviceType := range slices.Sorted(maps.Keys(want)) {
		if !slices.Equal(got[serviceType], want[serviceType]) {
			t.Errorf("%s jobs = %q, want %q", serviceType, got[serviceType], want[serviceType])
		}
	}
	for serviceType := range got {
		if _, ok := want[serviceType]; !ok {
			t.Errorf("unexpected jobs for %s: %q", serviceType, got[serviceType])
		}
	}
}
