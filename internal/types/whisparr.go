// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types //nolint:revive // pre-existing shared package name

// Whisparr V2 is a Sonarr V3 fork, so its queue payload currently matches the
// Sonarr shape. These types are deliberately kept separate from the Sonarr ones
// so that Sonarr-driven changes cannot silently alter Whisparr behaviour.

// WhisparrQueueResponse represents the queue response from the Whisparr API.
type WhisparrQueueResponse struct {
	Page          int                 `json:"page"`
	PageSize      int                 `json:"pageSize"`
	SortKey       string              `json:"sortKey"`
	SortDirection string              `json:"sortDirection"`
	TotalRecords  int                 `json:"totalRecords"`
	Records       []WhisparrQueueItem `json:"records"`
}

// WhisparrQueueItem represents a record in the Whisparr queue.
type WhisparrQueueItem struct {
	ID                      int                     `json:"id"`
	Title                   string                  `json:"title"`
	Status                  string                  `json:"status"`
	Size                    int64                   `json:"size"`
	SizeLeft                int64                   `json:"sizeleft"`
	TimeLeft                string                  `json:"timeleft,omitempty"`
	EstimatedCompletionTime string                  `json:"estimatedCompletionTime"`
	Added                   string                  `json:"added"`
	DownloadClient          string                  `json:"downloadClient"`
	DownloadID              string                  `json:"downloadId"`
	Protocol                string                  `json:"protocol"`
	Indexer                 string                  `json:"indexer"`
	OutputPath              string                  `json:"outputPath"`
	TrackedDownloadStatus   string                  `json:"trackedDownloadStatus"`
	TrackedDownloadState    string                  `json:"trackedDownloadState"`
	StatusMessages          []WhisparrStatusMessage `json:"statusMessages"`
	ErrorMessage            string                  `json:"errorMessage"`
	CustomFormatScore       int                     `json:"customFormatScore"`
	EpisodeID               int                     `json:"episodeId"`
	Episode                 WhisparrEpisode         `json:"episode"`
}

// WhisparrStatusMessage represents detailed status information for a queue record.
type WhisparrStatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// WhisparrEpisode represents the episode carried on a queue item. Whisparr
// models these as scenes, so there is no episode number: releases are
// identified by release date, and seasonNumber carries the release year.
// The API returns a single episode object per queue item, not an array.
type WhisparrEpisode struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	TvdbID        int    `json:"tvdbId"`
	Title         string `json:"title"`
	SeasonNumber  int    `json:"seasonNumber"`
	ReleaseDate   string `json:"releaseDate"`
	Runtime       int    `json:"runtime"`
	Overview      string `json:"overview"`
	HasFile       bool   `json:"hasFile"`
	EpisodeFileID int    `json:"episodeFileId"`
	Monitored     bool   `json:"monitored"`
}

// WhisparrQueueDeleteOptions represents options for deleting a queue item.
type WhisparrQueueDeleteOptions struct {
	RemoveFromClient bool `json:"removeFromClient"`
	Blocklist        bool `json:"blocklist"`
	SkipRedownload   bool `json:"skipRedownload"`
	ChangeCategory   bool `json:"changeCategory"`
}
