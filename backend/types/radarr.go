package types

// RadarrQueueResponse represents the queue response from Radarr API
type RadarrQueueResponse struct {
	Page          int                 `json:"page"`
	PageSize      int                 `json:"pageSize"`
	SortKey       string              `json:"sortKey"`
	SortDirection string              `json:"sortDirection"`
	TotalRecords  int                 `json:"totalRecords"`
	Records       []RadarrQueueRecord `json:"records"`
}

// RadarrQueueRecord represents a record in the Radarr queue
type RadarrQueueRecord struct {
	ID                      int                   `json:"id"`
	MovieID                 int                   `json:"movieId"`
	Title                   string                `json:"title"`
	Status                  string                `json:"status"`
	TimeLeft                string                `json:"timeleft,omitempty"`
	EstimatedCompletionTime string                `json:"estimatedCompletionTime"`
	Protocol                string                `json:"protocol"`
	Indexer                 string                `json:"indexer"`
	DownloadClient          string                `json:"downloadClient"`
	Size                    int64                 `json:"size"`
	SizeLeft                int64                 `json:"sizeleft"`
	CustomFormatScore       int                   `json:"customFormatScore"`
	TrackedDownloadStatus   string                `json:"trackedDownloadStatus"`
	TrackedDownloadState    string                `json:"trackedDownloadState"`
	StatusMessages          []RadarrStatusMessage `json:"statusMessages"`
	ErrorMessage            string                `json:"errorMessage"`
	DownloadId              string                `json:"downloadId"`
	Movie                   RadarrMovie           `json:"movie"`
}

// RadarrStatusMessage represents detailed status information for a queue record
type RadarrStatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// RadarrMovie represents the movie information in a queue record
type RadarrMovie struct {
	Title         string               `json:"title"`
	OriginalTitle string               `json:"originalTitle"`
	Year          int                  `json:"year"`
	FolderPath    string               `json:"folderPath"`
	CustomFormats []RadarrCustomFormat `json:"customFormats"`
}

// RadarrCustomFormat represents a custom format in Radarr
type RadarrCustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}