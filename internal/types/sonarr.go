// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

// SonarrQueueResponse represents the queue response from Sonarr API
type SonarrQueueResponse struct {
	Page          int           `json:"page"`
	PageSize      int           `json:"pageSize"`
	SortKey       string        `json:"sortKey"`
	SortDirection string        `json:"sortDirection"`
	TotalRecords  int           `json:"totalRecords"`
	Records       []QueueRecord `json:"records"`
}

// QueueRecord represents a record in the Sonarr queue
type QueueRecord struct {
	ID                      int             `json:"id"`
	Title                   string          `json:"title"`
	Status                  string          `json:"status"`
	TimeLeft                string          `json:"timeleft,omitempty"`
	EstimatedCompletionTime string          `json:"estimatedCompletionTime"`
	Indexer                 string          `json:"indexer"`
	DownloadClient          string          `json:"downloadClient"`
	Size                    int64           `json:"size"`
	SizeLeft                int64           `json:"sizeleft"`
	TrackedDownloadStatus   string          `json:"trackedDownloadStatus"`
	TrackedDownloadState    string          `json:"trackedDownloadState"`
	StatusMessages          []StatusMessage `json:"statusMessages"`
	ErrorMessage            string          `json:"errorMessage"`
	DownloadId              string          `json:"downloadId"`
	Protocol                string          `json:"protocol"`
}

// StatusMessage represents detailed status information for a queue record
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// SonarrStatsResponse represents the stats response from Sonarr API
type SonarrStatsResponse struct {
	MovieCount       int `json:"movieCount"`
	EpisodeCount     int `json:"episodeCount"`
	EpisodeFileCount int `json:"episodeFileCount"`
	Monitored        int `json:"monitored"`
	Unmonitored      int `json:"unmonitored"`
	QueuedCount      int `json:"queuedCount"`
	MissingCount     int `json:"missingCount"`
}

// SonarrSeriesResponse represents a series from Sonarr's series endpoint
type SonarrSeriesResponse struct {
	ID           int              `json:"id"`
	Title        string           `json:"title"`
	TitleSlug    string           `json:"titleSlug"`
	Overview     string           `json:"overview"`
	Status       string           `json:"status"`
	Added        string           `json:"added"`
	Year         int              `json:"year"`
	Path         string           `json:"path"`
	TvdbId       int              `json:"tvdbId"`
	ImdbId       string           `json:"imdbId"`
	SizeOnDisk   int64            `json:"sizeOnDisk"`
	Runtime      int              `json:"runtime"`
	Network      string           `json:"network"`
	AirTime      string           `json:"airTime"`
	Monitored    bool             `json:"monitored"`
	SeasonFolder bool             `json:"seasonFolder"`
	Seasons      []SonarrSeason   `json:"seasons"`
	Statistics   SeriesStatistics `json:"statistics"`
	Ratings      SeriesRatings    `json:"ratings"`
}

// SonarrSeason represents a season in a series
type SonarrSeason struct {
	SeasonNumber int     `json:"seasonNumber"`
	Monitored    bool    `json:"monitored"`
	Statistics   *Season `json:"statistics,omitempty"`
}

// Season represents season statistics
type Season struct {
	EpisodeFileCount  int     `json:"episodeFileCount"`
	EpisodeCount      int     `json:"episodeCount"`
	TotalEpisodeCount int     `json:"totalEpisodeCount"`
	SizeOnDisk        int64   `json:"sizeOnDisk"`
	PercentOfEpisodes float64 `json:"percentOfEpisodes"`
}

// SeriesStatistics represents series-wide statistics
type SeriesStatistics struct {
	SeasonCount       int     `json:"seasonCount"`
	EpisodeFileCount  int     `json:"episodeFileCount"`
	EpisodeCount      int     `json:"episodeCount"`
	TotalEpisodeCount int     `json:"totalEpisodeCount"`
	SizeOnDisk        int64   `json:"sizeOnDisk"`
	PercentOfEpisodes float64 `json:"percentOfEpisodes"`
}

// SeriesRatings represents rating information for a series
type SeriesRatings struct {
	Votes int     `json:"votes"`
	Value float64 `json:"value"`
}
