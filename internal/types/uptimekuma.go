// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type UptimeKumaMonitor struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	URL            string `json:"url,omitempty"`
	Status         string `json:"status"`
	ResponseTimeMs int64  `json:"responseTimeMs,omitempty"`
}

type UptimeKumaSummaryResponse struct {
	Monitors []UptimeKumaMonitor `json:"monitors"`
}
