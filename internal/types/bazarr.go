// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

// BazarrSystemStatusEnvelope represents Bazarr /api/system/status.
type BazarrSystemStatusEnvelope struct {
	Data BazarrSystemStatus `json:"data"`
}

// BazarrSystemStatus contains the status payload from Bazarr.
type BazarrSystemStatus struct {
	BazarrVersion string `json:"bazarr_version"`
}

// BazarrBadges represents Bazarr /api/badges.
type BazarrBadges struct {
	Episodes      int    `json:"episodes"`
	Movies        int    `json:"movies"`
	Providers     int    `json:"providers"`
	Status        int    `json:"status"`
	SonarrSignalR string `json:"sonarr_signalr"`
	RadarrSignalR string `json:"radarr_signalr"`
	Announcements int    `json:"announcements"`
}

// BazarrProviderStatus represents provider health from /api/providers.
type BazarrProviderStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Retry  string `json:"retry"`
}

// BazarrProvidersEnvelope represents Bazarr /api/providers response.
type BazarrProvidersEnvelope struct {
	Data []BazarrProviderStatus `json:"data"`
}

// BazarrHealthIssue represents a single system health issue.
type BazarrHealthIssue struct {
	Object string `json:"object"`
	Issue  string `json:"issue"`
}

// BazarrHealthIssuesEnvelope represents Bazarr /api/system/health response.
type BazarrHealthIssuesEnvelope struct {
	Data []BazarrHealthIssue `json:"data"`
}

// BazarrSummaryResponse is the combined payload used by API handlers and SSE.
type BazarrSummaryResponse struct {
	Badges       BazarrBadges           `json:"badges"`
	Providers    []BazarrProviderStatus `json:"providers"`
	HealthIssues []BazarrHealthIssue    `json:"healthIssues"`
}
