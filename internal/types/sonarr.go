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
