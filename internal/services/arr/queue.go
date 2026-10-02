// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
)

// App describes one *arr app. The name and the API version are the only
// values that differ between *arr apps.
type App struct {
	Name       string // service type, for example "sonarr"
	APIVersion string // path segment, for example "v3"
}

var (
	Sonarr   = App{Name: "sonarr", APIVersion: "v3"}
	Radarr   = App{Name: "radarr", APIVersion: "v3"}
	Lidarr   = App{Name: "lidarr", APIVersion: "v1"}
	Readarr  = App{Name: "readarr", APIVersion: "v1"}
	Whisparr = App{Name: "whisparr", APIVersion: "v3"}
)

// Apps holds one App for each *arr app.
var Apps = []App{Sonarr, Radarr, Lidarr, Readarr, Whisparr}

// AppFor returns the descriptor row for a service type. It returns false
// when the service type is not an *arr app.
func AppFor(serviceType string) (App, bool) {
	i := slices.IndexFunc(Apps, func(app App) bool { return app.Name == serviceType })
	if i < 0 {
		return App{}, false
	}
	return Apps[i], true
}

// QueuePage is the first page of the download queue of an *arr app.
// TotalRecords counts the full queue, not only the items in Records.
type QueuePage struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// QueueItem holds the fields that all *arr apps send with the same name.
type QueueItem struct {
	ID                      int             `json:"id"`
	Title                   string          `json:"title"`
	Status                  string          `json:"status"`
	Size                    int64           `json:"size"`
	SizeLeft                int64           `json:"sizeleft"`
	TimeLeft                string          `json:"timeleft"`
	EstimatedCompletionTime string          `json:"estimatedCompletionTime"`
	Protocol                string          `json:"protocol"`
	Indexer                 string          `json:"indexer"`
	DownloadClient          string          `json:"downloadClient"`
	CustomFormatScore       int             `json:"customFormatScore"`
	TrackedDownloadStatus   string          `json:"trackedDownloadStatus"`
	TrackedDownloadState    string          `json:"trackedDownloadState"`
	StatusMessages          []StatusMessage `json:"statusMessages"`
	ErrorMessage            string          `json:"errorMessage"`
	DownloadID              string          `json:"downloadId"`
}

type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

type QueueDeleteOptions struct {
	RemoveFromClient bool
	Blocklist        bool
	SkipRedownload   bool
	ChangeCategory   bool
}

func (a App) queueURL(baseURL string) string {
	return fmt.Sprintf("%s/api/%s/queue", strings.TrimRight(baseURL, "/"), a.APIVersion)
}

func (a App) requireConfig(op, baseURL, apiKey string) error {
	if baseURL == "" {
		return &ErrArr{Service: a.Name, Op: op, Err: errors.New("URL is required")}
	}
	if apiKey == "" {
		return &ErrArr{Service: a.Name, Op: op, Err: errors.New("API key is required")}
	}
	return nil
}

// FetchQueue returns the first page of the download queue.
func (a App) FetchQueue(ctx context.Context, baseURL, apiKey string) (QueuePage, error) {
	if err := a.requireConfig("get_queue", baseURL, apiKey); err != nil {
		return QueuePage{}, err
	}

	resp, err := MakeArrRequest(ctx, http.MethodGet, a.queueURL(baseURL)+"?page=1&pageSize=10", apiKey, nil)
	if err != nil {
		return QueuePage{}, &ErrArr{Service: a.Name, Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return QueuePage{}, &ErrArr{Service: a.Name, Op: "get_queue", HttpCode: resp.StatusCode}
	}

	var page QueuePage
	if err := json.UnmarshalRead(resp.Body, &page); err != nil {
		return QueuePage{}, &ErrArr{Service: a.Name, Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}
	if page.Records == nil {
		page.Records = []QueueItem{}
	}

	return page, nil
}

// DeleteQueueItem removes one queue item. For a download that holds several
// queue items, the *arr app removes the whole download.
func (a App) DeleteQueueItem(ctx context.Context, baseURL, apiKey, queueID string, opts QueueDeleteOptions) error {
	if err := a.requireConfig("delete_queue", baseURL, apiKey); err != nil {
		return err
	}

	deleteURL := fmt.Sprintf("%s/%s?removeFromClient=%t&blocklist=%t&skipRedownload=%t",
		a.queueURL(baseURL),
		url.PathEscape(queueID),
		opts.RemoveFromClient,
		opts.Blocklist,
		opts.SkipRedownload,
	)
	if opts.ChangeCategory {
		deleteURL += "&changeCategory=true"
	}

	log.Info().
		Str("service", a.Name).
		Str("url", deleteURL).
		Str("queueId", queueID).
		Bool("removeFromClient", opts.RemoveFromClient).
		Bool("blocklist", opts.Blocklist).
		Bool("skipRedownload", opts.SkipRedownload).
		Bool("changeCategory", opts.ChangeCategory).
		Msg("Attempting to delete queue item")

	resp, err := MakeArrRequest(ctx, http.MethodDelete, deleteURL, apiKey, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("service", a.Name).
			Str("url", deleteURL).
			Str("queueId", queueID).
			Msg("Failed to execute delete request")
		return &ErrArr{Service: a.Name, Op: "delete_queue", Err: fmt.Errorf("failed to execute request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		log.Error().
			Str("service", a.Name).
			Int("statusCode", resp.StatusCode).
			Str("url", deleteURL).
			Str("queueId", queueID).
			Str("response", string(body)).
			Msg("Delete request failed")

		if msg := ExtractMessageField(body); msg != "" {
			return &ErrArr{Service: a.Name, Op: "delete_queue", Err: errors.New(msg), HttpCode: resp.StatusCode}
		}
		return &ErrArr{Service: a.Name, Op: "delete_queue", HttpCode: resp.StatusCode}
	}

	log.Info().
		Str("service", a.Name).
		Str("queueId", queueID).
		Msg("Successfully deleted queue item")

	return nil
}
