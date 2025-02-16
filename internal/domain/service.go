// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package domain

import (
	"strings"
	"time"
)

type ServiceType string

const (
	ServiceTypeAutobrr     ServiceType = "AUTOBRR"
	ServiceTypeRadarr      ServiceType = "RADARR"
	ServiceTypeSonarr      ServiceType = "SONARR"
	ServiceTypeProwlarr    ServiceType = "PROWLARR"
	ServiceTypeOverseerr   ServiceType = "OVERSEERR"
	ServiceTypePlex        ServiceType = "PLEX"
	ServiceTypeTailscale   ServiceType = "TAILSCALE"
	ServiceTypeMaintainerr ServiceType = "MAINTAINERR"
	ServiceTypeGeneral     ServiceType = "GENERAL"
)

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) FromString(str string) ServiceType {
	if str == "" {
	}
	return ServiceType(str)
}

func (s ServiceType) ParseString(str string) ServiceType {
	if strings.Contains(str, "-") {
		parts := strings.Split(str, "-")
		if len(parts) == 2 {
			return ServiceType(strings.ToUpper(parts[0]))
		}
	}

	return "INVALID"
}

// Service represents a configured service instance
type Service struct {
	ID             string      `json:"id"`
	Type           ServiceType `json:"type"`
	URL            string      `json:"url"`
	AccessURL      string      `json:"accessUrl,omitempty"` // New field for external access URL
	APIKey         string      `json:"apiKey,omitempty"`
	Name           string      `json:"name"`
	DisplayName    string      `json:"displayName,omitempty"`
	HealthEndpoint string      `json:"healthEndpoint,omitempty"`
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status          string                 `json:"status"`
	ResponseTime    int64                  `json:"responseTime"`
	LastChecked     time.Time              `json:"lastChecked"`
	Message         string                 `json:"message,omitempty"`
	Version         string                 `json:"version,omitempty"`
	UpdateAvailable bool                   `json:"updateAvailable,omitempty"`
	ServiceID       string                 `json:"serviceId"`
	Stats           map[string]interface{} `json:"stats,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

type ServiceConfigResponse struct {
	InstanceID  string `json:"instanceId"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey,omitempty"`
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

type WebhookProxyRequest struct {
	TargetUrl string `json:"targetUrl"`
	APIKey    string `json:"apiKey"`
}

type Changes struct {
	New   []string `json:"new"`
	Fixed []string `json:"fixed"`
}

type FindServiceParams struct {
	InstanceID     string
	InstancePrefix string
	URL            string
	AccessURL      string
}
