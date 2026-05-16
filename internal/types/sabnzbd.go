// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import "encoding/json"

// FlexString unmarshals from either a JSON string or a JSON number.
// Sabnzbd changed several count fields (noofslots_total, etc.) from
// quoted strings to bare numbers between API versions.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexString(n.String())
	return nil
}

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
	HaveWarnings    FlexString         `json:"have_warnings"`
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
