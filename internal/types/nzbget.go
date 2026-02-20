// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type NzbgetStatus struct {
	RemainingSizeLo uint64 `json:"RemainingSizeLo"`
	RemainingSizeHi uint64 `json:"RemainingSizeHi"`
	RemainingSizeMB int64  `json:"RemainingSizeMB"`

	DownloadRate   int64  `json:"DownloadRate"`
	DownloadRateLo uint64 `json:"DownloadRateLo"`
	DownloadRateHi uint64 `json:"DownloadRateHi"`

	DownloadPaused bool `json:"DownloadPaused"`
	PostPaused     bool `json:"PostPaused"`
	ScanPaused     bool `json:"ScanPaused"`
	ServerStandBy  bool `json:"ServerStandBy"`
	QuotaReached   bool `json:"QuotaReached"`

	ServerTime int64 `json:"ServerTime"`
	ResumeTime int64 `json:"ResumeTime"`

	FreeDiskSpaceLo  uint64 `json:"FreeDiskSpaceLo"`
	FreeDiskSpaceHi  uint64 `json:"FreeDiskSpaceHi"`
	FreeDiskSpaceMB  int64  `json:"FreeDiskSpaceMB"`
	TotalDiskSpaceLo uint64 `json:"TotalDiskSpaceLo"`
	TotalDiskSpaceHi uint64 `json:"TotalDiskSpaceHi"`
	TotalDiskSpaceMB int64  `json:"TotalDiskSpaceMB"`
}

type NzbgetQueueItem struct {
	NZBID            int64  `json:"NZBID"`
	NZBName          string `json:"NZBName"`
	Category         string `json:"Category"`
	Status           string `json:"Status"`
	RemainingSizeLo  uint64 `json:"RemainingSizeLo"`
	RemainingSizeHi  uint64 `json:"RemainingSizeHi"`
	RemainingSizeMB  int64  `json:"RemainingSizeMB"`
	DownloadedSizeMB int64  `json:"DownloadedSizeMB"`
	DownloadTimeSec  int64  `json:"DownloadTimeSec"`
	Health           int64  `json:"Health"`
	CriticalHealth   int64  `json:"CriticalHealth"`
}

type NzbgetHistoryItem struct {
	NZBID            int64  `json:"NZBID"`
	Kind             string `json:"Kind"`
	Name             string `json:"Name"`
	NZBName          string `json:"NZBName"`
	Category         string `json:"Category"`
	Status           string `json:"Status"`
	HistoryTime      int64  `json:"HistoryTime"`
	FileSizeMB       int64  `json:"FileSizeMB"`
	DownloadedSizeMB int64  `json:"DownloadedSizeMB"`
	DownloadTimeSec  int64  `json:"DownloadTimeSec"`
}

type NzbgetSummaryResponse struct {
	Status         NzbgetStatus        `json:"status"`
	Queue          []NzbgetQueueItem   `json:"queue"`
	FailedCount    int                 `json:"failedCount"`
	RecentFailures []NzbgetHistoryItem `json:"recentFailures"`
}
