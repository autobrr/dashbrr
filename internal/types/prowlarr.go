// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type ProwlarrStatsResponse struct {
	GrabCount    int `json:"grabCount"`
	FailCount    int `json:"failCount"`
	IndexerCount int `json:"indexerCount"`
}

type ProwlarrIndexer struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Label               string `json:"label"`
	Enable              bool   `json:"enable"`
	Priority            int    `json:"priority"`
	AverageResponseTime int    `json:"averageResponseTime"`
	NumberOfGrabs       int    `json:"numberOfGrabs"`
	NumberOfQueries     int    `json:"numberOfQueries"`
}

type ProwlarrIndexerStats struct {
	ID                        int    `json:"id"`
	IndexerID                 int    `json:"indexerId"`
	IndexerName               string `json:"indexerName"`
	AverageResponseTime       int    `json:"averageResponseTime"`
	NumberOfQueries           int    `json:"numberOfQueries"`
	NumberOfGrabs             int    `json:"numberOfGrabs"`
	NumberOfRssQueries        int    `json:"numberOfRssQueries"`
	NumberOfAuthQueries       int    `json:"numberOfAuthQueries"`
	NumberOfFailedQueries     int    `json:"numberOfFailedQueries"`
	NumberOfFailedGrabs       int    `json:"numberOfFailedGrabs"`
	NumberOfFailedRssQueries  int    `json:"numberOfFailedRssQueries"`
	NumberOfFailedAuthQueries int    `json:"numberOfFailedAuthQueries"`
}

type ProwlarrIndexerStatsResponse struct {
	Indexers []ProwlarrIndexerStats `json:"indexers"`
}
