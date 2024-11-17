// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import "time"

type StatusResponse struct {
	Version         string `json:"version"`
	CommitTag       string `json:"commitTag"`
	Status          int    `json:"status"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

type RequestsResponse struct {
	PageInfo struct {
		Pages    int `json:"pages"`
		PageSize int `json:"pageSize"`
		Results  int `json:"results"`
		Page     int `json:"page"`
	} `json:"pageInfo"`
	Results []MediaRequest `json:"results"`
}

type MediaRequest struct {
	ID        int       `json:"id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Media     struct {
		ID                int      `json:"id"`
		TmdbID            int      `json:"tmdbId"`
		TvdbID            int      `json:"tvdbId"`
		Status            int      `json:"status"`
		Requests          []string `json:"requests"`
		CreatedAt         string   `json:"createdAt"`
		UpdatedAt         string   `json:"updatedAt"`
		MediaType         string   `json:"mediaType"`
		ServiceUrl        string   `json:"serviceUrl"`
		Title             string   `json:"title,omitempty"`
		ExternalServiceID int      `json:"externalServiceId,omitempty"`
	} `json:"media"`
	RequestedBy struct {
		ID           int    `json:"id"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		PlexToken    string `json:"plexToken"`
		PlexUsername string `json:"plexUsername"`
		UserType     int    `json:"userType"`
		Permissions  int    `json:"permissions"`
		Avatar       string `json:"avatar"`
		CreatedAt    string `json:"createdAt"`
		UpdatedAt    string `json:"updatedAt"`
		RequestCount int    `json:"requestCount"`
	} `json:"requestedBy"`
	ModifiedBy struct {
		ID           int    `json:"id"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		PlexToken    string `json:"plexToken"`
		PlexUsername string `json:"plexUsername"`
		UserType     int    `json:"userType"`
		Permissions  int    `json:"permissions"`
		Avatar       string `json:"avatar"`
		CreatedAt    string `json:"createdAt"`
		UpdatedAt    string `json:"updatedAt"`
		RequestCount int    `json:"requestCount"`
	} `json:"modifiedBy"`
	Is4k       bool   `json:"is4k"`
	ServerID   int    `json:"serverId"`
	ProfileID  int    `json:"profileId"`
	RootFolder string `json:"rootFolder"`
}

type RequestsStats struct {
	PendingCount int            `json:"pendingCount"`
	Requests     []MediaRequest `json:"requests"`
}

type OverseerrStats struct {
	Requests     []MediaRequest `json:"requests"`
	PendingCount int            `json:"pendingCount"`
}

type OverseerrDetails struct {
	PendingCount  int `json:"pendingCount"`
	TotalRequests int `json:"totalRequests"`
}

type OverseerrServiceHealth struct {
	ServiceID   string           `json:"serviceId"`
	Status      string           `json:"status"`
	Message     string           `json:"message"`
	LastChecked time.Time        `json:"lastChecked"`
	Stats       OverseerrStats   `json:"stats"`
	Details     OverseerrDetails `json:"details"`
}
