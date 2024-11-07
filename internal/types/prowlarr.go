package types

// ProwlarrIndexer represents an indexer in Prowlarr
type ProwlarrIndexer struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Enable   bool   `json:"enable"`
	Priority int    `json:"priority"`
}

// ProwlarrStatsResponse represents the stats response from Prowlarr API
type ProwlarrStatsResponse struct {
	GrabCount    int `json:"grabCount"`
	FailCount    int `json:"failCount"`
	IndexerCount int `json:"indexerCount"`
}

// ProwlarrStats represents the combined stats for Prowlarr
type ProwlarrStats struct {
	Stats    ProwlarrStatsResponse `json:"stats"`
	Indexers []ProwlarrIndexer     `json:"indexers"`
}
