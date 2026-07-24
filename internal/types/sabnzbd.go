// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type SabnzbdVersionEnvelope struct {
	Version string `json:"version"`
}

type SabnzbdQueueEnvelope struct {
	Queue SabnzbdQueue `json:"queue"`
}

type SabnzbdQueue struct {
	Version         string             `json:"version"`
	Status          string             `json:"status"`
	Paused          bool               `json:"paused"`
	Speed           string             `json:"speed"`
	KBPerSec        string             `json:"kbpersec"`
	TimeLeft        string             `json:"timeleft"`
	SizeLeft        string             `json:"sizeleft"`
	Size            string             `json:"size"`
	MBLeft          string             `json:"mbleft"`
	MB              string             `json:"mb"`
	NoOfSlots       FlexString         `json:"noofslots"`
	NoOfSlotsTotal  FlexString         `json:"noofslots_total"`
	Diskspace1      string             `json:"diskspace1"`
	Diskspace2      string             `json:"diskspace2"`
	DiskspaceTotal1 string             `json:"diskspacetotal1"`
	DiskspaceTotal2 string             `json:"diskspacetotal2"`
	Diskspace1Norm  string             `json:"diskspace1_norm"`
	Diskspace2Norm  string             `json:"diskspace2_norm"`
	HaveWarnings    string             `json:"have_warnings"`
	SpeedLimitAbs   string             `json:"speedlimit_abs"`
	Slots           []SabnzbdQueueSlot `json:"slots"`
}

type SabnzbdQueueSlot struct {
	NzoID      string `json:"nzo_id"`
	Filename   string `json:"filename"`
	Status     string `json:"status"`
	Size       string `json:"size"`
	SizeLeft   string `json:"sizeleft"`
	Percentage string `json:"percentage"`
	TimeLeft   string `json:"timeleft"`
	Category   string `json:"cat"`
	Priority   string `json:"priority"`
}

type SabnzbdHistoryEnvelope struct {
	History SabnzbdHistory `json:"history"`
}

type SabnzbdHistory struct {
	NoOfSlots int                  `json:"noofslots"`
	Slots     []SabnzbdHistorySlot `json:"slots"`
}

type SabnzbdHistorySlot struct {
	NzoID       string `json:"nzo_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	FailMessage string `json:"fail_message"`
	Category    string `json:"category"`
	Size        string `json:"size"`
	Completed   int64  `json:"completed"`
}

type SabnzbdSummaryResponse struct {
	Queue          SabnzbdQueue         `json:"queue"`
	FailedCount    int                  `json:"failedCount"`
	RecentFailures []SabnzbdHistorySlot `json:"recentFailures"`
}
