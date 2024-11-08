// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import (
	"encoding/xml"
	"time"
)

const (
	PlexSessionCacheDuration = 30 * time.Second
)

type MediaContainer struct {
	XMLName  xml.Name `xml:"MediaContainer"`
	Version  string   `xml:"version,attr"`
	Platform string   `xml:"platform,attr"`
}

type PlexResponse struct {
	MediaContainer MediaContainer `json:"MediaContainer"`
}

type PlexSessionsResponse struct {
	MediaContainer struct {
		Size     int           `json:"size"`
		Metadata []PlexSession `json:"Metadata"`
	} `json:"MediaContainer"`
}

type PlexUser struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Thumb string `json:"thumb"`
}

type PlexPlayer struct {
	Address             string `json:"address"`
	Device              string `json:"device"`
	MachineIdentifier   string `json:"machineIdentifier"`
	Model               string `json:"model"`
	Platform            string `json:"platform"`
	PlatformVersion     string `json:"platformVersion"`
	Product             string `json:"product"`
	Profile             string `json:"profile"`
	State               string `json:"state"`
	RemotePublicAddress string `json:"remotePublicAddress"`
	Title               string `json:"title"`
	Vendor              string `json:"vendor"`
	Version             string `json:"version"`
}

type PlexSession struct {
	AddedAt              int64            `json:"addedAt"`
	Art                  string           `json:"art"`
	Duration             int              `json:"duration"`
	GrandparentArt       string           `json:"grandparentArt"`
	GrandparentGuid      string           `json:"grandparentGuid"`
	GrandparentKey       string           `json:"grandparentKey"`
	GrandparentRatingKey string           `json:"grandparentRatingKey"`
	GrandparentThumb     string           `json:"grandparentThumb"`
	GrandparentTitle     string           `json:"grandparentTitle"`
	Guid                 string           `json:"guid"`
	Index                int              `json:"index"`
	Key                  string           `json:"key"`
	LibrarySectionID     string           `json:"librarySectionID"`
	LibrarySectionKey    string           `json:"librarySectionKey"`
	LibrarySectionTitle  string           `json:"librarySectionTitle"`
	MusicAnalysisVersion string           `json:"musicAnalysisVersion"`
	ParentGuid           string           `json:"parentGuid"`
	ParentIndex          int              `json:"parentIndex"`
	ParentKey            string           `json:"parentKey"`
	ParentRatingKey      string           `json:"parentRatingKey"`
	ParentStudio         string           `json:"parentStudio"`
	ParentThumb          string           `json:"parentThumb"`
	ParentTitle          string           `json:"parentTitle"`
	ParentYear           int              `json:"parentYear"`
	RatingCount          int              `json:"ratingCount"`
	RatingKey            string           `json:"ratingKey"`
	SessionKey           string           `json:"sessionKey"`
	Thumb                string           `json:"thumb"`
	Title                string           `json:"title"`
	TitleSort            string           `json:"titleSort"`
	Type                 string           `json:"type"`
	UpdatedAt            int64            `json:"updatedAt"`
	ViewOffset           int              `json:"viewOffset"`
	Media                []PlexMedia      `json:"Media"`
	User                 *PlexUser        `json:"User,omitempty"`
	Player               *PlexPlayer      `json:"Player,omitempty"`
	Session              *PlexSessionInfo `json:"Session,omitempty"`
}

type PlexMedia struct {
	AudioChannels int        `json:"audioChannels"`
	AudioCodec    string     `json:"audioCodec"`
	Bitrate       int        `json:"bitrate"`
	Container     string     `json:"container"`
	Duration      int        `json:"duration"`
	ID            string     `json:"id"`
	Selected      bool       `json:"selected"`
	Part          []PlexPart `json:"Part"`
}

type PlexPart struct {
	Container    string       `json:"container"`
	Duration     int          `json:"duration"`
	File         string       `json:"file"`
	HasThumbnail string       `json:"hasThumbnail"`
	ID           string       `json:"id"`
	Key          string       `json:"key"`
	Size         int64        `json:"size"`
	Decision     string       `json:"decision"`
	Selected     bool         `json:"selected"`
	Stream       []PlexStream `json:"Stream"`
}

type PlexStream struct {
	AlbumGain            string `json:"albumGain"`
	AlbumPeak            string `json:"albumPeak"`
	AlbumRange           string `json:"albumRange"`
	AudioChannelLayout   string `json:"audioChannelLayout"`
	BitDepth             int    `json:"bitDepth"`
	Bitrate              int    `json:"bitrate"`
	Channels             int    `json:"channels"`
	Codec                string `json:"codec"`
	DisplayTitle         string `json:"displayTitle"`
	ExtendedDisplayTitle string `json:"extendedDisplayTitle"`
	Gain                 string `json:"gain"`
	ID                   string `json:"id"`
	Index                int    `json:"index"`
	Loudness             string `json:"loudness"`
	LRA                  string `json:"lra"`
	Peak                 string `json:"peak"`
	SamplingRate         int    `json:"samplingRate"`
	Selected             bool   `json:"selected"`
	StreamType           int    `json:"streamType"`
	Location             string `json:"location"`
}

type PlexSessionInfo struct {
	ID        string `json:"id"`
	Bandwidth int    `json:"bandwidth"`
	Location  string `json:"location"`
}
