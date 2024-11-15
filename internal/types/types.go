// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import "time"

type ServiceHealth struct {
	Status          string    `json:"status"`
	ResponseTime    int64     `json:"responseTime"`
	LastChecked     time.Time `json:"lastChecked"`
	Message         string    `json:"message"`
	UpdateAvailable bool      `json:"updateAvailable"`
	Version         string    `json:"version,omitempty"`
}

type ServiceConfigResponse struct {
	InstanceID  string `json:"instanceId"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey,omitempty"`
}

type WebhookProxyRequest struct {
	TargetUrl string `json:"targetUrl"`
	APIKey    string `json:"apiKey"`
}

type UpdateResponse struct {
	Version     string    `json:"version"`
	Branch      string    `json:"branch"`
	ReleaseDate time.Time `json:"releaseDate"`
	FileName    string    `json:"fileName"`
	URL         string    `json:"url"`
	Installed   bool      `json:"installed"`
	InstalledOn time.Time `json:"installedOn"`
	Installable bool      `json:"installable"`
	Latest      bool      `json:"latest"`
	Changes     Changes   `json:"changes"`
	Hash        string    `json:"hash"`
}

type Changes struct {
	New   []string `json:"new"`
	Fixed []string `json:"fixed"`
}

type FindUserParams struct {
	ID       int64
	Username string
	Email    string
}

type FindServiceParams struct {
	InstanceID     string
	InstancePrefix string
	URL            string
	AccessURL      string
}
