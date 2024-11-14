// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package autobrr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

type AutobrrService struct {
	core.ServiceCore
}

type AutobrrStats struct {
	TotalCount          int `json:"total_count"`
	FilteredCount       int `json:"filtered_count"`
	FilterRejectedCount int `json:"filter_rejected_count"`
	PushApprovedCount   int `json:"push_approved_count"`
	PushRejectedCount   int `json:"push_rejected_count"`
	PushErrorCount      int `json:"push_error_count"`
}

type IRCStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Enabled bool   `json:"enabled"`
}

type VersionResponse struct {
	Version string `json:"version"`
}

type ReleasesResponse struct {
	Data       []Release `json:"data"`
	Count      int       `json:"count"`
	NextCursor int       `json:"next_cursor"`
}

type Release struct {
	ID           int            `json:"id"`
	FilterStatus string         `json:"filter_status"`
	Rejections   []string       `json:"rejections"`
	Indexer      Indexer        `json:"indexer"`
	Filter       string         `json:"filter"`
	Protocol     string         `json:"protocol"`
	Timestamp    time.Time      `json:"timestamp"`
	InfoURL      string         `json:"info_url"`
	DownloadURL  string         `json:"download_url"`
	Name         string         `json:"name"`
	Size         int64          `json:"size"`
	Title        string         `json:"title"`
	Category     string         `json:"category"`
	Season       int            `json:"season"`
	Episode      int            `json:"episode"`
	Year         int            `json:"year"`
	Resolution   string         `json:"resolution"`
	Source       string         `json:"source"`
	Codec        []string       `json:"codec"`
	Container    string         `json:"container"`
	HDR          []string       `json:"hdr"`
	Group        string         `json:"group"`
	Type         string         `json:"type"`
	Origin       string         `json:"origin"`
	ActionStatus []ActionStatus `json:"action_status"`
}

type Indexer struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Identifier         string `json:"identifier"`
	IdentifierExternal string `json:"identifier_external"`
}

type ActionStatus struct {
	ID         int       `json:"id"`
	Status     string    `json:"status"`
	Action     string    `json:"action"`
	ActionID   int       `json:"action_id"`
	Type       string    `json:"type"`
	Client     string    `json:"client"`
	Filter     string    `json:"filter"`
	FilterID   int       `json:"filter_id"`
	Rejections []string  `json:"rejections"`
	ReleaseID  int       `json:"release_id"`
	Timestamp  time.Time `json:"timestamp"`
}

func init() {
	models.NewAutobrrService = NewAutobrrService
}

func NewAutobrrService() models.ServiceHealthChecker {
	service := &AutobrrService{}
	service.Type = "autobrr"
	service.DisplayName = "Autobrr"
	service.Description = "Monitor and manage your Autobrr instance"
	service.DefaultURL = "http://localhost:7474"
	service.HealthEndpoint = "/api/healthz/liveness"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *AutobrrService) getEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s%s", baseURL, path)
}

func (s *AutobrrService) GetReleases(ctx context.Context, url, apiKey string) (ReleasesResponse, error) {
	if url == "" || apiKey == "" {
		return ReleasesResponse{}, fmt.Errorf("service not configured: missing URL or API key")
	}

	releasesURL := s.getEndpoint(url, "/api/release")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, releasesURL, apiKey, headers)
	if err != nil {
		return ReleasesResponse{}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ReleasesResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return ReleasesResponse{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var releases ReleasesResponse
	if err := json.Unmarshal(body, &releases); err != nil {
		return ReleasesResponse{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return releases, nil
}

func (s *AutobrrService) GetReleaseStats(ctx context.Context, url, apiKey string) (AutobrrStats, error) {
	if url == "" || apiKey == "" {
		return AutobrrStats{}, fmt.Errorf("service not configured: missing URL or API key")
	}

	statsURL := s.getEndpoint(url, "/api/release/stats")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, statsURL, apiKey, headers)
	if err != nil {
		return AutobrrStats{}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AutobrrStats{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return AutobrrStats{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var stats AutobrrStats
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()

	if err := decoder.Decode(&stats); err != nil {
		return AutobrrStats{}, fmt.Errorf("failed to decode response: %v, body: %s", err, string(body))
	}

	return stats, nil
}

func (s *AutobrrService) GetIRCStatusFromCache(url string) string {
	if status := s.GetVersionFromCache(url + "_irc"); status != "" {
		return status
	}
	return ""
}

func (s *AutobrrService) CacheIRCStatus(url, status string) error {
	return s.CacheVersion(url+"_irc", status, 5*time.Minute)
}

func (s *AutobrrService) GetIRCStatus(ctx context.Context, url, apiKey string) ([]IRCStatus, error) {
	if url == "" || apiKey == "" {
		return nil, fmt.Errorf("service not configured: missing URL or API key")
	}

	// Check cache first
	if cached := s.GetIRCStatusFromCache(url); cached != "" {
		var status []IRCStatus
		if err := json.Unmarshal([]byte(cached), &status); err == nil {
			return status, nil
		}
	}

	ircURL := s.getEndpoint(url, "/api/irc")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, ircURL, apiKey, headers)
	if err != nil {
		return []IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return []IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("failed to read response body: %v", err)
	}

	// Try to decode as array first
	var allStatus []IRCStatus
	if err := json.Unmarshal(body, &allStatus); err == nil {
		var unhealthyStatus []IRCStatus
		for _, status := range allStatus {
			if !status.Healthy && status.Enabled {
				unhealthyStatus = append(unhealthyStatus, status)
			}
		}
		// Cache the result
		if cached, err := json.Marshal(unhealthyStatus); err == nil {
			if err := s.CacheIRCStatus(url, string(cached)); err != nil {
				fmt.Printf("Failed to cache IRC status: %v\n", err)
			}
		}
		return unhealthyStatus, nil
	}

	// If array decode fails, try to decode as single object
	var singleStatus IRCStatus
	if err := json.Unmarshal(body, &singleStatus); err == nil {
		// Only return if unhealthy AND enabled
		if !singleStatus.Healthy && singleStatus.Enabled {
			status := []IRCStatus{singleStatus}
			// Cache the result
			if cached, err := json.Marshal(status); err == nil {
				if err := s.CacheIRCStatus(url, string(cached)); err != nil {
					fmt.Printf("Failed to cache IRC status: %v\n", err)
				}
			}
			return status, nil
		}
		// Cache empty result
		if err := s.CacheIRCStatus(url, "[]"); err != nil {
			fmt.Printf("Failed to cache IRC status: %v\n", err)
		}
		return []IRCStatus{}, nil
	}

	return []IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("failed to decode response: %s", string(body))
}

func (s *AutobrrService) GetVersion(ctx context.Context, url, apiKey string) (string, error) {
	// Check cache first
	if version := s.GetVersionFromCache(url); version != "" {
		return version, nil
	}

	versionURL := s.getEndpoint(url, "/api/config")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, versionURL, apiKey, headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", err
	}

	var versionData VersionResponse
	if err := json.Unmarshal(body, &versionData); err != nil {
		return "", err
	}

	// Cache version for 2 hours to align with update check
	if err := s.CacheVersion(url, versionData.Version, 2*time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return versionData.Version, nil
}

func (s *AutobrrService) GetUpdateFromCache(url string) string {
	if update := s.GetVersionFromCache(url + "_update"); update != "" {
		return update
	}
	return ""
}

func (s *AutobrrService) CacheUpdate(url, status string, ttl time.Duration) error {
	return s.CacheVersion(url+"_update", status, ttl)
}

func (s *AutobrrService) CheckUpdate(ctx context.Context, url, apiKey string) (bool, error) {
	// Check cache first
	if status := s.GetUpdateFromCache(url); status != "" {
		return status == "true", nil
	}

	updateURL := s.getEndpoint(url, "/api/updates/latest")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, updateURL, apiKey, headers)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 200 means update available, 204 means no update
	hasUpdate := resp.StatusCode == http.StatusOK
	status := "false"
	if hasUpdate {
		status = "true"
	}

	// Cache result for 2 hours to match autobrr's check interval
	if err := s.CacheUpdate(url, status, 2*time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache update status: %v\n", err)
	}

	return hasUpdate, nil
}

func (s *AutobrrService) CheckHealth(ctx context.Context, url string, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" || apiKey == "" {
		return s.CreateHealthResponse(startTime, "pending", "Autobrr not configured"), http.StatusOK
	}

	// Create a context with timeout for the entire health check
	ctx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	// Start version check in background
	versionChan := make(chan string, 1)
	versionErrChan := make(chan error, 1)
	go func() {
		version, err := s.GetVersion(ctx, url, apiKey)
		if err != nil {
			versionErrChan <- err
			versionChan <- ""
			return
		}
		versionChan <- version
		versionErrChan <- nil
	}()

	// Start update check in background
	updateChan := make(chan bool, 1)
	updateErrChan := make(chan error, 1)
	go func() {
		hasUpdate, err := s.CheckUpdate(ctx, url, apiKey)
		if err != nil {
			updateErrChan <- err
			updateChan <- false
			return
		}
		updateChan <- hasUpdate
		updateErrChan <- nil
	}()

	// Get release stats
	stats, err := s.GetReleaseStats(ctx, url, apiKey)
	if err != nil {
		fmt.Printf("Failed to get release stats: %v\n", err)
		// Continue without stats, don't fail the health check
	}

	// Perform health check
	livenessURL := s.getEndpoint(url, "/api/healthz/liveness")
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, livenessURL, apiKey, headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusOK
	}
	defer resp.Body.Close()

	// Calculate response time directly
	responseTime := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)), http.StatusOK
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to read response: %v", err)), http.StatusOK
	}

	trimmedBody := strings.TrimSpace(string(body))
	trimmedBody = strings.Trim(trimmedBody, "\"")

	if trimmedBody != "healthy" && trimmedBody != "OK" {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Autobrr reported unhealthy status: %s", trimmedBody)), http.StatusOK
	}

	// Wait for version and update status with timeout
	var version string
	var versionErr error
	var hasUpdate bool
	var updateErr error

	select {
	case version = <-versionChan:
		versionErr = <-versionErrChan
	case <-ctx.Done():
		versionErr = ctx.Err()
	}

	select {
	case hasUpdate = <-updateChan:
		updateErr = <-updateErrChan
	case <-ctx.Done():
		updateErr = ctx.Err()
	}

	// Get IRC status
	ircStatus, err := s.GetIRCStatus(ctx, url, apiKey)
	if err != nil {
		extras := map[string]interface{}{
			"responseTime": responseTime,
			"stats": map[string]interface{}{
				"autobrr": stats,
			},
			"details": map[string]interface{}{
				"autobrr": map[string]interface{}{
					"irc": ircStatus,
				},
			},
		}
		if version != "" {
			extras["version"] = version
		}
		if versionErr != nil {
			extras["versionError"] = versionErr.Error()
		}
		if !hasUpdate && updateErr != nil {
			extras["updateError"] = updateErr.Error()
		} else {
			extras["updateAvailable"] = hasUpdate
		}

		return s.CreateHealthResponse(startTime, "warning", fmt.Sprintf("Autobrr is running but IRC status check failed: %v", err), extras), http.StatusOK
	}

	// Check if any IRC connections are healthy
	ircHealthy := false

	// If no IRC networks are configured, consider it healthy and continue
	if len(ircStatus) == 0 {
		ircHealthy = true
	} else {
		for _, status := range ircStatus {
			if status.Healthy {
				ircHealthy = true
				break
			}
		}
	}

	extras := map[string]interface{}{
		"responseTime": responseTime,
		"stats": map[string]interface{}{
			"autobrr": stats,
		},
	}

	if version != "" {
		extras["version"] = version
	}
	if versionErr != nil {
		extras["versionError"] = versionErr.Error()
	}
	if !hasUpdate && updateErr != nil {
		extras["updateError"] = updateErr.Error()
	} else {
		extras["updateAvailable"] = hasUpdate
	}

	// Only include IRC status in details if there are unhealthy connections
	if !ircHealthy {
		extras["details"] = map[string]interface{}{
			"autobrr": map[string]interface{}{
				"irc": ircStatus,
			},
		}
		return s.CreateHealthResponse(startTime, "warning", "Autobrr is running but reports unhealthy IRC connections", extras), http.StatusOK
	}

	return s.CreateHealthResponse(startTime, "online", "Autobrr is running", extras), http.StatusOK
}
