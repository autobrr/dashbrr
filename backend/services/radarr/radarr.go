package radarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/backend/models"
	"github.com/autobrr/dashbrr/backend/services/base"
)

const (
	queueTimeout = 5 * time.Second
)

type RadarrService struct {
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

type QueueResponse struct {
	Page          int           `json:"page"`
	PageSize      int           `json:"pageSize"`
	SortKey       string        `json:"sortKey"`
	SortDirection string        `json:"sortDirection"`
	TotalRecords  int           `json:"totalRecords"`
	Records       []QueueRecord `json:"records"`
}

type QueueRecord struct {
	MovieID        int       `json:"movieId"`
	Title          string    `json:"title"`
	Status         string    `json:"status"`
	TimeLeft       string    `json:"timeleft"`
	EstimatedTime  time.Time `json:"estimatedCompletionTime"`
	Protocol       string    `json:"protocol"`
	DownloadClient string    `json:"downloadClient"`
	Size           int64     `json:"size"`
	SizeLeft       int64     `json:"sizeleft"`
}

func init() {
	models.NewRadarrService = func() models.ServiceHealthChecker {
		return &RadarrService{}
	}
}

func (s *RadarrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/v3/health", baseURL)
}

func (s *RadarrService) getSystemStatus(baseURL, apiKey string) (string, error) {
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

func (s *RadarrService) GetQueue(baseURL, apiKey string) (*QueueResponse, error) {
	queueURL := fmt.Sprintf("%s/api/v3/queue", strings.TrimRight(baseURL, "/"))

	ctx, cancel := context.WithTimeout(context.Background(), queueTimeout)
	defer cancel()

	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   queueTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", queueURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("received nil response from server")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var queueResp QueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
		return nil, err
	}

	return &queueResp, nil
}

func (s *RadarrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
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

	extras := map[string]interface{}{
		"responseTime": responseTime.Milliseconds(),
	}
	if version != "" {
		extras["version"] = version
	}

	// If there are no health issues, the service is healthy
	if len(healthIssues) == 0 {
		return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
	}

	// Process health issues
	var messages []string
	for _, issue := range healthIssues {
		message := issue.Message
		message = strings.TrimPrefix(message, "IndexerStatusCheck: ")
		message = strings.TrimPrefix(message, "ApplicationLongTermStatusCheck: ")
		messages = append(messages, message)
	}

	return s.CreateHealthResponse(startTime, "warning", strings.Join(messages, "; "), extras), http.StatusOK
}
