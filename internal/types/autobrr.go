package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type AutobrrStats struct {
	TotalCount          int `json:"total_count"`
	FilteredCount       int `json:"filtered_count"`
	FilterRejectedCount int `json:"filter_rejected_count"`
	PushApprovedCount   int `json:"push_approved_count"`
	PushRejectedCount   int `json:"push_rejected_count"`
	PushErrorCount      int `json:"push_error_count"`
}

type IRCStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Enabled bool   `json:"enabled"`
}

type VersionResponse struct {
	Version string `json:"version"`
}

type ReleasesResponse struct {
	Data       []Release `json:"data"`
	Count      int       `json:"count"`
	NextCursor int       `json:"next_cursor"`
}

// ReleaseType is a custom type that can handle both string and number types
type ReleaseType string

func (rt *ReleaseType) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*rt = ReleaseType(s)
		return nil
	}

	// Try number
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*rt = ReleaseType(strconv.Itoa(n))
		return nil
	}

	return fmt.Errorf("type must be string or number")
}

type Release struct {
	ID              int            `json:"id"`
	FilterStatus    string         `json:"filter_status"`
	Rejections      []string       `json:"rejections"`
	Indexer         Indexer        `json:"indexer"`
	Filter          string         `json:"filter"`
	Protocol        string         `json:"protocol"`
	Implementation  string         `json:"implementation"`
	Timestamp       time.Time      `json:"timestamp"`
	Type            ReleaseType    `json:"type"`
	InfoURL         string         `json:"info_url"`
	DownloadURL     string         `json:"download_url"`
	GroupID         string         `json:"group_id"`
	TorrentID       string         `json:"torrent_id"`
	Name            string         `json:"name"`
	NormalizedHash  string         `json:"normalized_hash"`
	Size            int64          `json:"size"`
	Title           string         `json:"title"`
	SubTitle        string         `json:"sub_title"`
	Category        string         `json:"category"`
	Season          int            `json:"season"`
	Episode         int            `json:"episode"`
	Year            int            `json:"year"`
	Month           int            `json:"month"`
	Day             int            `json:"day"`
	Resolution      string         `json:"resolution"`
	Source          string         `json:"source"`
	Codec           []string       `json:"codec"`
	Container       string         `json:"container"`
	HDR             []string       `json:"hdr"`
	Group           string         `json:"group"`
	Proper          bool           `json:"proper"`
	Repack          bool           `json:"repack"`
	Website         string         `json:"website"`
	Hybrid          bool           `json:"hybrid"`
	Edition         []string       `json:"edition"`
	Cut             []string       `json:"cut"`
	MediaProcessing string         `json:"media_processing"`
	Origin          string         `json:"origin"`
	Uploader        string         `json:"uploader"`
	PreTime         string         `json:"pre_time"`
	ActionStatus    []ActionStatus `json:"action_status"`
}

type Indexer struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Identifier         string `json:"identifier"`
	IdentifierExternal string `json:"identifier_external"`
}

type ActionStatus struct {
	ID         int       `json:"id"`
	Status     string    `json:"status"`
	Action     string    `json:"action"`
	ActionID   int       `json:"action_id"`
	Type       string    `json:"type"`
	Client     string    `json:"client"`
	Filter     string    `json:"filter"`
	FilterID   int       `json:"filter_id"`
	Rejections []string  `json:"rejections"`
	ReleaseID  int       `json:"release_id"`
	Timestamp  time.Time `json:"timestamp"`
}
