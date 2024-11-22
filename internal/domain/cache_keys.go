package domain

import "time"

const (
	CacheKeyProwlarrStatsPrefix        = "prowlarr:stats:"
	CacheKeyProwlarrIndexerPrefix      = "prowlarr:indexers:"
	CacheKeyProwlarrIndexerStatsPrefix = "prowlarr:indexerstats:"
	CacheKeyProwlarrStaleDataDuration  = 5 * time.Minute // How long to serve stale data
)
