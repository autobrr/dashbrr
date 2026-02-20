// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type JellyfinSystemInfo struct {
	ServerName  string `json:"ServerName"`
	Version     string `json:"Version"`
	ProductName string `json:"ProductName"`
	ID          string `json:"Id"`
}

type JellyfinNowPlayingItem struct {
	Name         string                `json:"Name"`
	SeriesName   string                `json:"SeriesName"`
	Type         string                `json:"Type"`
	RunTimeTicks int64                 `json:"RunTimeTicks"`
	MediaStreams []JellyfinMediaStream `json:"MediaStreams"`
}

type JellyfinMediaStream struct {
	Codec    string `json:"Codec"`
	BitRate  int64  `json:"BitRate"`
	Channels int    `json:"Channels"`
	Index    int    `json:"Index"`
	Width    int    `json:"Width"`
	Height   int    `json:"Height"`
}

type JellyfinPlayerState struct {
	PositionTicks    int64  `json:"PositionTicks"`
	IsPaused         bool   `json:"IsPaused"`
	PlayMethod       string `json:"PlayMethod"`
	AudioStreamIndex *int   `json:"AudioStreamIndex"`
}

type JellyfinTranscodingInfo struct {
	AudioCodec           string  `json:"AudioCodec"`
	VideoCodec           string  `json:"VideoCodec"`
	Container            string  `json:"Container"`
	IsVideoDirect        bool    `json:"IsVideoDirect"`
	IsAudioDirect        bool    `json:"IsAudioDirect"`
	Bitrate              int64   `json:"Bitrate"`
	CompletionPercentage float64 `json:"CompletionPercentage"`
	Width                int     `json:"Width"`
	Height               int     `json:"Height"`
	AudioChannels        int     `json:"AudioChannels"`
}

type JellyfinSession struct {
	ID                 string                   `json:"Id"`
	UserName           string                   `json:"UserName"`
	Client             string                   `json:"Client"`
	DeviceName         string                   `json:"DeviceName"`
	DeviceType         string                   `json:"DeviceType"`
	ApplicationVersion string                   `json:"ApplicationVersion"`
	IsActive           bool                     `json:"IsActive"`
	PlayState          *JellyfinPlayerState     `json:"PlayState"`
	NowPlayingItem     *JellyfinNowPlayingItem  `json:"NowPlayingItem"`
	TranscodingInfo    *JellyfinTranscodingInfo `json:"TranscodingInfo"`
}

type JellyfinSummaryResponse struct {
	System   JellyfinSystemInfo `json:"system"`
	Sessions []JellyfinSession  `json:"sessions"`
}
