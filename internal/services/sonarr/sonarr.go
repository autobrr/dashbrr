// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sonarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/base"
	"github.com/autobrr/dashbrr/internal/types"
)

type SonarrService struct {
	base.BaseService
}

type HealthResponse struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

type SystemStatusResponse struct {
	Version string `json:"version"`
}

func init() {
	models.NewSonarrService = NewSonarrService
}

func NewSonarrService() models.ServiceHealthChecker {
	service := &SonarrService{}
	service.Type = "sonarr"
	service.DisplayName = "Sonarr"
	service.Description = "Monitor and manage your Sonarr instance"
	service.DefaultURL = "http://localhost:8989"
	service.HealthEndpoint = "/api/v3/health"
	return service
}

func (s *SonarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

func (s *SonarrService) getSystemStatus(baseURL, apiKey string) (string, error) {
	// Check cache first
	if version := s.GetVersionFromCache(baseURL); version != "" {
		return version, nil
	}

	statusURL := fmt.Sprintf("%s/api/v3/system/status", strings.TrimRight(baseURL, "/"))
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.MakeRequestWithContext(ctx, statusURL, "", headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", err
	}

	var status SystemStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return "", err
	}

	// Cache version for 1 hour
	if err := s.CacheVersion(baseURL, status.Version, time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return status.Version, nil
}

func (s *SonarrService) checkForUpdates(baseURL, apiKey string) (bool, error) {
	updateURL := fmt.Sprintf("%s/api/v3/update", strings.TrimRight(baseURL, "/"))
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := s.MakeRequestWithContext(ctx, updateURL, "", headers)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return false, err
	}

	var updates []types.UpdateResponse
	if err := json.Unmarshal(body, &updates); err != nil {
		return false, err
	}

	// Check if there's any update available
	for _, update := range updates {
		if !update.Installed && update.Installable {
			return true, nil
		}
	}

	return false, nil
}

func (s *SonarrService) getQueue(url, apiKey string) ([]types.QueueRecord, error) {
	if url == "" || apiKey == "" {
		return nil, fmt.Errorf("service not configured: missing URL or API key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	queueURL := fmt.Sprintf("%s/api/v3/queue", strings.TrimRight(url, "/"))
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, queueURL, apiKey, headers)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var queue types.SonarrQueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return queue.Records, nil
}

func (s *SonarrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	// Create a context with timeout for the entire health check
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start version check in background
	versionChan := make(chan string, 1)
	go func() {
		version, err := s.getSystemStatus(url, apiKey)
		if err != nil {
			versionChan <- ""
			return
		}
		versionChan <- version
	}()

	// Start update check in background
	updateChan := make(chan bool, 1)
	go func() {
		hasUpdate, err := s.checkForUpdates(url, apiKey)
		if err != nil {
			updateChan <- false
			return
		}
		updateChan <- hasUpdate
	}()

	// Start queue check in background
	queueChan := make(chan []types.QueueRecord, 1)
	go func() {
		queue, err := s.getQueue(url, apiKey)
		if err != nil {
			queueChan <- nil
			return
		}
		queueChan <- queue
	}()

	// Perform health check
	healthEndpoint := s.GetHealthEndpoint(url)
	headers := map[string]string{
		"auth_header": "X-Api-Key",
		"auth_value":  apiKey,
	}

	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", headers)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Health check failed: %v", err)), http.StatusServiceUnavailable
	}
	defer resp.Body.Close()

	// Get response time from header
	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to read response: %v", err)), http.StatusInternalServerError
	}

	var healthIssues []HealthResponse
	if err := json.Unmarshal(body, &healthIssues); err != nil {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to parse response: %v", err)), http.StatusInternalServerError
	}

	// Wait for version with timeout
	var version string
	select {
	case v := <-versionChan:
		version = v
	case <-time.After(500 * time.Millisecond):
		// Continue without version if it takes too long
	}

	// Wait for update check with timeout
	var updateAvailable bool
	select {
	case u := <-updateChan:
		updateAvailable = u
	case <-time.After(500 * time.Millisecond):
		// Continue without update check if it takes too long
	}

	// Wait for queue with timeout
	var queue []types.QueueRecord
	select {
	case q := <-queueChan:
		queue = q
	case <-time.After(500 * time.Millisecond):
		// Continue without queue if it takes too long
	}

	extras := map[string]interface{}{
		"responseTime": responseTime.Milliseconds(),
	}
	if version != "" {
		extras["version"] = version
	}
	if updateAvailable {
		extras["updateAvailable"] = true
	}

	var allWarnings []string

	// Process health issues first
	if len(healthIssues) > 0 {
		var indexerWarnings []string
		var otherWarnings []string

		for _, issue := range healthIssues {
			message := issue.Message
			message = strings.TrimPrefix(message, "IndexerStatusCheck: ")
			message = strings.TrimPrefix(message, "ApplicationLongTermStatusCheck: ")

			if strings.Contains(message, "Indexers unavailable due to failures") {
				// Extract indexer names from the message
				parts := strings.Split(message, ":")
				if len(parts) > 1 {
					indexers := strings.Split(parts[1], ",")
					for _, indexer := range indexers {
						indexer = strings.TrimSpace(indexer)
						if indexer != "" {
							indexerWarnings = append(indexerWarnings, fmt.Sprintf("- %s", indexer))
						}
					}
				}
			} else {
				otherWarnings = append(otherWarnings, fmt.Sprintf("- %s", message))
			}
		}

		if len(indexerWarnings) > 0 {
			allWarnings = append(allWarnings, fmt.Sprintf("Indexers unavailable due to failures:\n%s", strings.Join(indexerWarnings, "\n")))
		}
		if len(otherWarnings) > 0 {
			allWarnings = append(allWarnings, strings.Join(otherWarnings, "\n"))
		}
	}

	// Check queue for warning status
	if queue != nil {
		type releaseWarnings struct {
			title    string
			messages []string
			status   string
		}
		warningsByRelease := make(map[string]*releaseWarnings)

		for _, record := range queue {
			if record.TrackedDownloadStatus == "warning" {
				warnings, exists := warningsByRelease[record.Title]
				if !exists {
					warnings = &releaseWarnings{
						title:    record.Title,
						messages: []string{},
						status:   record.Status,
					}
					warningsByRelease[record.Title] = warnings
				}

				// Collect all status messages
				for _, msg := range record.StatusMessages {
					if msg.Title != "" && !strings.Contains(msg.Title, record.Title) {
						warnings.messages = append(warnings.messages, msg.Title)
					}
					warnings.messages = append(warnings.messages, msg.Messages...)
				}
			}
		}

		if len(warningsByRelease) > 0 {
			var warningMessages []string
			for _, warnings := range warningsByRelease {
				var messages []string
				for _, msg := range warnings.messages {
					messages = append(messages, fmt.Sprintf("- %s", msg))
				}
				message := fmt.Sprintf("\n%s:\n%s", warnings.title, strings.Join(messages, "\n"))
				warningMessages = append(warningMessages, message)
			}
			allWarnings = append(allWarnings, fmt.Sprintf("Queue warnings:%s", strings.Join(warningMessages, "")))
		}
	}

	// If there are any warnings, return them all
	if len(allWarnings) > 0 {
		return s.CreateHealthResponse(startTime, "warning", strings.Join(allWarnings, "\n\n"), extras), http.StatusOK
	}

	// If no warnings, the service is healthy
	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}
