// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

// ReadarrQueueResponse represents the queue response from Readarr API.
type ReadarrQueueResponse struct {
	Page          int                `json:"page"`
	PageSize      int                `json:"pageSize"`
	SortKey       string             `json:"sortKey"`
	SortDirection string             `json:"sortDirection"`
	TotalRecords  int                `json:"totalRecords"`
	Records       []ReadarrQueueItem `json:"records"`
}

// ReadarrQueueItem represents a record in the Readarr queue.
type ReadarrQueueItem struct {
	ID                    int                    `json:"id"`
	Title                 string                 `json:"title"`
	Status                string                 `json:"status"`
	TimeLeft              string                 `json:"timeleft,omitempty"`
	EstimatedCompletionAt string                 `json:"estimatedCompletionTime"`
	Protocol              string                 `json:"protocol"`
	Indexer               string                 `json:"indexer"`
	DownloadClient        string                 `json:"downloadClient"`
	Size                  int64                  `json:"size"`
	SizeLeft              int64                  `json:"sizeleft"`
	CustomFormatScore     int                    `json:"customFormatScore"`
	TrackedDownloadStatus string                 `json:"trackedDownloadStatus"`
	TrackedDownloadState  string                 `json:"trackedDownloadState"`
	StatusMessages        []ReadarrStatusMessage `json:"statusMessages"`
	ErrorMessage          string                 `json:"errorMessage"`
	DownloadID            string                 `json:"downloadId"`
}

// ReadarrStatusMessage represents detailed status information for a queue record.
type ReadarrStatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// ReadarrQueueDeleteOptions represents options for deleting a queue item.
type ReadarrQueueDeleteOptions struct {
	RemoveFromClient bool `json:"removeFromClient"`
	Blocklist        bool `json:"blocklist"`
	SkipRedownload   bool `json:"skipRedownload"`
	ChangeCategory   bool `json:"changeCategory"`
}
