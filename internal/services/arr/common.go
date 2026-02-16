// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/core"
)

// Custom error type for *arr services
type ErrArr struct {
	Service  string // Service name (e.g., "radarr", "sonarr")
	Op       string // Operation that failed
	Err      error  // Underlying error
	HttpCode int    // HTTP status code if applicable
}

func (e *ErrArr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("%s %s: server returned %s (%d)", e.Service, e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s %s: %v", e.Service, e.Op, e.Err)
	}
	return fmt.Sprintf("%s %s", e.Service, e.Op)
}

func (e *ErrArr) Unwrap() error {
	return e.Err
}

type SystemStatusResponse struct {
	Version string `json:"version"`
}

var arrHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// MakeArrRequest is a helper function to make requests with proper headers
func MakeArrRequest(ctx context.Context, method, url, apiKey string, body []byte) (*http.Response, error) {
	// If no deadline is set, apply a default to avoid hanging requests.
	reqCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		reqCtx, cancel = context.WithTimeout(ctx, core.DefaultTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set headers correctly
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	// Track request start time
	startTime := time.Now()

	resp, err := arrHTTPClient.Do(req.WithContext(reqCtx))
	if err != nil {
		if err == context.Canceled {
			return nil, fmt.Errorf("request canceled: %w", err)
		}
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("received nil response from server")
	}

	// Store the response time in milliseconds
	resp.Header.Set("X-Response-Time", fmt.Sprintf("%d", time.Since(startTime).Milliseconds()))

	return resp, nil
}

// GetArrSystemStatus provides a common implementation for getting system status.
func GetArrSystemStatus(ctx context.Context, service, url, apiKey string, getVersionFromCache func(string) string, cacheVersion func(string, string, time.Duration) error) (string, error) {
	if url == "" {
		return "", &ErrArr{Service: service, Op: "get_system_status", Err: fmt.Errorf("URL is required")}
	}

	// Check cache first using version-specific cache key
	if version := getVersionFromCache(url); version != "" && version != "true" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v3/system/status", strings.TrimRight(url, "/"))
	ctx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	resp, err := MakeArrRequest(ctx, http.MethodGet, statusURL, apiKey, nil)
	if err != nil {
		return "", &ErrArr{Service: service, Op: "get_system_status", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrArr{Service: service, Op: "get_system_status", HttpCode: resp.StatusCode}
	}

	var status SystemStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", &ErrArr{Service: service, Op: "get_system_status", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Cache version for 1 hour
	if err := cacheVersion(url, status.Version, time.Hour); err != nil {
		log.Debug().Err(err).Str("url", url).Str("service", service).Msg("Failed to cache version")
	}

	return status.Version, nil
}

// CheckArrForUpdates provides a common implementation for checking updates
func CheckArrForUpdates(ctx context.Context, service, url, apiKey string) (bool, error) {
	if url == "" {
		return false, &ErrArr{Service: service, Op: "check_for_updates", Err: fmt.Errorf("URL is required")}
	}

	updateURL := fmt.Sprintf("%s/api/v3/update", strings.TrimRight(url, "/"))
	ctx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	resp, err := MakeArrRequest(ctx, http.MethodGet, updateURL, apiKey, nil)
	if err != nil {
		return false, &ErrArr{Service: service, Op: "check_for_updates", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, &ErrArr{Service: service, Op: "check_for_updates", HttpCode: resp.StatusCode}
	}

	type UpdateResponse struct {
		Installed   bool `json:"installed"`
		Installable bool `json:"installable"`
	}

	var updates []UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return false, &ErrArr{Service: service, Op: "check_for_updates", Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	// Check if there's any update available
	for _, update := range updates {
		if !update.Installed && update.Installable {
			return true, nil
		}
	}

	return false, nil
}
