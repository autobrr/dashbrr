// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import "time"

type QuiHealthResponse struct {
	Status string `json:"status"`
}

type QuiInstance struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Connected          bool   `json:"connected"`
	IsActive           bool   `json:"isActive"`
	ConnectionStatus   string `json:"connectionStatus,omitempty"`
	HasDecryptionError bool   `json:"hasDecryptionError,omitempty"`
}

type QuiTransferInfo struct {
	ConnectionStatus string `json:"connection_status"`
	DHTNodes         int    `json:"dht_nodes"`
	Downloaded       int64  `json:"dl_info_data"`
	DownloadSpeed    int64  `json:"dl_info_speed"`
	DownloadRate     int64  `json:"dl_rate_limit"`
	Uploaded         int64  `json:"up_info_data"`
	UploadSpeed      int64  `json:"up_info_speed"`
	UploadRate       int64  `json:"up_rate_limit"`
}

type QuiInstanceTransfer struct {
	InstanceID       int    `json:"instanceId"`
	Name             string `json:"name"`
	Connected        bool   `json:"connected"`
	Active           bool   `json:"active"`
	ConnectionStatus string `json:"connectionStatus,omitempty"`
	Downloaded       int64  `json:"downloaded"`
	Uploaded         int64  `json:"uploaded"`
	DownloadSpeed    int64  `json:"downloadSpeed"`
	UploadSpeed      int64  `json:"uploadSpeed"`
	DHTNodes         int    `json:"dhtNodes"`
}

type QuiTransferSummary struct {
	TotalInstances     int   `json:"totalInstances"`
	ActiveInstances    int   `json:"activeInstances"`
	ConnectedInstances int   `json:"connectedInstances"`
	DownloadSpeed      int64 `json:"downloadSpeed"`
	UploadSpeed        int64 `json:"uploadSpeed"`
	Downloaded         int64 `json:"downloaded"`
	Uploaded           int64 `json:"uploaded"`
	DHTNodes           int   `json:"dhtNodes"`
}

type QuiCrossSeedSettings struct {
	Enabled            bool `json:"enabled"`
	RunIntervalMinutes int  `json:"runIntervalMinutes"`
}

type QuiCrossSeedRun struct {
	ID              int64      `json:"id"`
	Status          string     `json:"status"`
	Mode            string     `json:"mode"`
	TriggeredBy     string     `json:"triggeredBy"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	CandidatesFound int        `json:"candidatesFound"`
	TorrentsAdded   int        `json:"torrentsAdded"`
	TorrentsFailed  int        `json:"torrentsFailed"`
	TorrentsSkipped int        `json:"torrentsSkipped"`
	Message         string     `json:"message,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
}

type QuiCrossSeedStatus struct {
	Settings  *QuiCrossSeedSettings `json:"settings,omitempty"`
	LastRun   *QuiCrossSeedRun      `json:"lastRun,omitempty"`
	NextRunAt *time.Time            `json:"nextRunAt,omitempty"`
	Running   bool                  `json:"running"`
}
