// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type QueueDeleteOptions struct {
	RemoveFromClient bool
	Blocklist        bool
	SkipRedownload   bool
	ChangeCategory   bool
}

func BuildQueueDeleteURL(baseURL, queueID string, opts QueueDeleteOptions) string {
	baseURL = strings.TrimRight(baseURL, "/")

	deleteURL := fmt.Sprintf("%s/api/v3/queue/%s?removeFromClient=%t&blocklist=%t&skipRedownload=%t",
		baseURL,
		queueID,
		opts.RemoveFromClient,
		opts.Blocklist,
		opts.SkipRedownload,
	)

	if opts.ChangeCategory {
		deleteURL += "&changeCategory=true"
	}

	return deleteURL
}

func BuildQueueURL(baseURL, rawQuery string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	queueURL := fmt.Sprintf("%s/api/v3/queue", baseURL)
	query := strings.TrimLeft(rawQuery, "?")
	if query != "" {
		queueURL += "?" + query
	}
	return queueURL
}

func FetchQueueBody(
	ctx context.Context,
	service, baseURL, apiKey, rawQuery string,
	readBody func(*http.Response) ([]byte, error),
) ([]byte, error) {
	if baseURL == "" {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("URL is required")}
	}

	if apiKey == "" {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("API key is required")}
	}

	if readBody == nil {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("readBody function is required")}
	}

	queueURL := BuildQueueURL(baseURL, rawQuery)
	resp, err := MakeArrRequest(ctx, http.MethodGet, queueURL, apiKey, nil)
	if err != nil {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrArr{Service: service, Op: "get_queue", HttpCode: resp.StatusCode}
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	return body, nil
}

func FetchQueueRecords[T any](
	ctx context.Context,
	service, baseURL, apiKey, rawQuery string,
	readBody func(*http.Response) ([]byte, error),
) ([]T, error) {
	body, err := FetchQueueBody(ctx, service, baseURL, apiKey, rawQuery, readBody)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Records []T `json:"records"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &ErrArr{Service: service, Op: "get_queue", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	if payload.Records == nil {
		return []T{}, nil
	}

	return payload.Records, nil
}
