// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

// SonarrUpdateResponse represents an update response from Sonarr
type SonarrUpdateResponse struct {
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Installable bool   `json:"installable"`
}
