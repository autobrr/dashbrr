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

// SonarrQueueDeleteOptions represents the options for deleting a queue item in Sonarr
type SonarrQueueDeleteOptions struct {
	RemoveFromClient bool `json:"removeFromClient"`
	Blocklist        bool `json:"blocklist"`
	SkipRedownload   bool `json:"skipRedownload"`
	ChangeCategory   bool `json:"changeCategory"`
}

// QueueRecord represents a record in the Sonarr queue
type QueueRecord struct {
	ID                                  int             `json:"id"`
	SeriesID                            int             `json:"seriesId"`
	EpisodeID                           int             `json:"episodeId"`
	SeasonNumber                        int             `json:"seasonNumber"`
	Series                              Series          `json:"series"`
	Episode                             Episode         `json:"episode"`
	Title                               string          `json:"title"`
	Status                              string          `json:"status"`
	Size                                int64           `json:"size"`
	SizeLeft                            int64           `json:"sizeleft"`
	TimeLeft                            string          `json:"timeleft,omitempty"`
	EstimatedCompletionTime             string          `json:"estimatedCompletionTime"`
	Added                               string          `json:"added"`
	DownloadClient                      string          `json:"downloadClient"`
	DownloadID                          string          `json:"downloadId"`
	Protocol                            string          `json:"protocol"`
	Indexer                             string          `json:"indexer"`
	OutputPath                          string          `json:"outputPath"`
	TrackedDownloadStatus               string          `json:"trackedDownloadStatus"`
	TrackedDownloadState                string          `json:"trackedDownloadState"`
	StatusMessages                      []StatusMessage `json:"statusMessages"`
	ErrorMessage                        string          `json:"errorMessage"`
	DownloadClientHasPostImportCategory bool            `json:"downloadClientHasPostImportCategory"`
	EpisodeHasFile                      bool            `json:"episodeHasFile"`
	CustomFormatScore                   int             `json:"customFormatScore"`
	Episodes                            []EpisodeBasic  `json:"episodes"`
}

// Series represents a TV series in Sonarr
type Series struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Path             string `json:"path"`
	Year             int    `json:"year"`
	Status           string `json:"status"`
	Overview         string `json:"overview"`
	Network          string `json:"network"`
	AirTime          string `json:"airTime"`
	Monitored        bool   `json:"monitored"`
	QualityProfileID int    `json:"qualityProfileId"`
	SeasonFolder     bool   `json:"seasonFolder"`
	Runtime          int    `json:"runtime"`
	TvdbID           int    `json:"tvdbId"`
	TvRageID         int    `json:"tvRageId"`
	TvMazeID         int    `json:"tvMazeId"`
	FirstAired       string `json:"firstAired"`
	LastAired        string `json:"lastAired"`
	SeriesType       string `json:"seriesType"`
	CleanTitle       string `json:"cleanTitle"`
	ImdbID           string `json:"imdbId"`
	TitleSlug        string `json:"titleSlug"`
}

// Episode represents an episode in Sonarr
type Episode struct {
	ID                       int    `json:"id"`
	SeriesID                 int    `json:"seriesId"`
	EpisodeNumber            int    `json:"episodeNumber"`
	SeasonNumber             int    `json:"seasonNumber"`
	Title                    string `json:"title"`
	AirDate                  string `json:"airDate"`
	AirDateUTC               string `json:"airDateUtc"`
	Overview                 string `json:"overview"`
	HasFile                  bool   `json:"hasFile"`
	Monitored                bool   `json:"monitored"`
	AbsoluteEpisodeNumber    int    `json:"absoluteEpisodeNumber"`
	UnverifiedSceneNumbering bool   `json:"unverifiedSceneNumbering"`
}

// EpisodeBasic represents a basic episode structure for queue items
type EpisodeBasic struct {
	ID            int `json:"id"`
	EpisodeNumber int `json:"episodeNumber"`
	SeasonNumber  int `json:"seasonNumber"`
}

// StatusMessage represents a status message in the queue
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// SonarrStatsResponse represents the stats response from Sonarr API
type SonarrStatsResponse struct {
	MovieCount       int   `json:"movieCount"`
	EpisodeCount     int   `json:"episodeCount"`
	EpisodeFileCount int   `json:"episodeFileCount"`
	FreeSpaceBytes   int64 `json:"freeSpaceBytes"`
	TotalSpaceBytes  int64 `json:"totalSpaceBytes"`
	Monitored        int   `json:"monitored"`
	Unmonitored      int   `json:"unmonitored"`
	QueuedCount      int   `json:"queuedCount"`
	MissingCount     int   `json:"missingCount"`
}

// SonarrUpdateResponse represents an update response from Sonarr
type SonarrUpdateResponse struct {
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Installable bool   `json:"installable"`
}
