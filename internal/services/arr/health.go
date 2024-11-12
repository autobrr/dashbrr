// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// HealthResponse represents a common health check response structure
type HealthResponse struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

// HealthChecker interface defines methods required for health checking
type HealthChecker interface {
	GetSystemStatus(url, apiKey string) (string, error)
	CheckForUpdates(url, apiKey string) (bool, error)
	GetHealthEndpoint(baseURL string) string
}

// healthCheckResult holds the results of individual health check operations
type healthCheckResult struct {
	data interface{}
	err  error
}

// instanceHealthCheck manages health checks for a single instance
type instanceHealthCheck struct {
	url       string
	apiKey    string
	startTime time.Time
	checker   HealthChecker
	core      *core.ServiceCore
	ctx       context.Context

	// Channels for different health check operations
	version chan healthCheckResult
	update  chan healthCheckResult
	health  chan healthCheckResult
}

// newInstanceHealthCheck creates a new instance health check
func newInstanceHealthCheck(ctx context.Context, s *core.ServiceCore, url, apiKey string, checker HealthChecker) *instanceHealthCheck {
	return &instanceHealthCheck{
		url:       url,
		apiKey:    apiKey,
		startTime: time.Now(),
		checker:   checker,
		core:      s,
		ctx:       ctx,
		version:   make(chan healthCheckResult, 1),
		update:    make(chan healthCheckResult, 1),
		health:    make(chan healthCheckResult, 1),
	}
}

// runChecks executes all health check operations concurrently
func (ihc *instanceHealthCheck) runChecks() {
	var wg sync.WaitGroup

	// Version check
	wg.Add(1)
	go func() {
		defer wg.Done()
		version, err := ihc.checker.GetSystemStatus(ihc.url, ihc.apiKey)
		ihc.version <- healthCheckResult{data: version, err: err}
	}()

	// Update check
	wg.Add(1)
	go func() {
		defer wg.Done()
		hasUpdate, err := ihc.checker.CheckForUpdates(ihc.url, ihc.apiKey)
		ihc.update <- healthCheckResult{data: hasUpdate, err: err}
	}()

	// Health check
	wg.Add(1)
	go func() {
		defer wg.Done()
		healthEndpoint := ihc.checker.GetHealthEndpoint(ihc.url)
		headers := map[string]string{
			"X-Api-Key": ihc.apiKey,
		}
		resp, err := ihc.core.MakeRequestWithContext(ihc.ctx, healthEndpoint, ihc.apiKey, headers)
		if err != nil {
			ihc.health <- healthCheckResult{err: err}
			return
		}

		if resp == nil {
			ihc.health <- healthCheckResult{err: fmt.Errorf("nil response")}
			return
		}

		defer resp.Body.Close()
		body, err := ihc.core.ReadBody(resp)

		// Parse response time from header
		respTimeStr := resp.Header.Get("X-Response-Time")
		var respTime time.Duration
		if respTimeStr != "" {
			respTime, _ = time.ParseDuration(respTimeStr)
		}

		ihc.health <- healthCheckResult{data: struct {
			body       []byte
			statusCode int
			respTime   time.Duration
		}{
			body:       body,
			statusCode: resp.StatusCode,
			respTime:   respTime,
		}, err: err}
	}()

	// Wait for all goroutines to complete
	wg.Wait()
}

// processHealthResponse processes the health check response
func processHealthResponse(body []byte) ([]HealthResponse, error) {
	var healthIssues []HealthResponse
	if err := json.Unmarshal(body, &healthIssues); err != nil {
		return nil, err
	}
	return healthIssues, nil
}

// ArrHealthCheck provides a common implementation of health checking for *arr services
func ArrHealthCheck(s *core.ServiceCore, url, apiKey string, checker HealthChecker) (models.ServiceHealth, int) {
	if url == "" {
		return s.CreateHealthResponse(time.Now(), "error", "URL is required"), http.StatusBadRequest
	}

	// Create a context with a single timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create and run instance health check
	ihc := newInstanceHealthCheck(ctx, s, url, apiKey, checker)
	ihc.runChecks()

	// Process results
	extras := make(map[string]interface{})
	var healthStatus string
	var healthMessage string
	var allWarnings []string

	// Process health check results
	select {
	case healthResult := <-ihc.health:
		if healthResult.err != nil {
			return s.CreateHealthResponse(ihc.startTime, "offline", fmt.Sprintf("Failed to connect: %v", healthResult.err)), http.StatusOK
		}

		if data, ok := healthResult.data.(struct {
			body       []byte
			statusCode int
			respTime   time.Duration
		}); ok {
			extras["responseTime"] = data.respTime.Milliseconds()

			if data.statusCode >= 400 {
				statusText := http.StatusText(data.statusCode)
				switch data.statusCode {
				case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
					return s.CreateHealthResponse(ihc.startTime, "error", fmt.Sprintf("Service is temporarily unavailable (%d %s)", data.statusCode, statusText)), http.StatusOK
				case http.StatusUnauthorized:
					return s.CreateHealthResponse(ihc.startTime, "error", "Invalid API key"), http.StatusOK
				case http.StatusForbidden:
					return s.CreateHealthResponse(ihc.startTime, "error", "Access forbidden"), http.StatusOK
				case http.StatusNotFound:
					return s.CreateHealthResponse(ihc.startTime, "error", "Service endpoint not found"), http.StatusOK
				default:
					return s.CreateHealthResponse(ihc.startTime, "error", fmt.Sprintf("Server returned %s (%d)", statusText, data.statusCode)), http.StatusOK
				}
			}

			if healthIssues, err := processHealthResponse(data.body); err == nil {
				for _, issue := range healthIssues {
					if issue.Type == "warning" || issue.Type == "error" {
						warning := formatWarning(issue)
						allWarnings = append(allWarnings, warning)
					}
				}
			}
		}
	case <-ctx.Done():
		return s.CreateHealthResponse(ihc.startTime, "error", "Health check timed out"), http.StatusOK
	}

	// Process version check
	select {
	case versionResult := <-ihc.version:
		if versionResult.err == nil {
			if version, ok := versionResult.data.(string); ok && version != "" {
				extras["version"] = version
			}
		}
	case <-ctx.Done():
		// Continue without version
	}

	// Process update check
	select {
	case updateResult := <-ihc.update:
		if updateResult.err == nil {
			if hasUpdate, ok := updateResult.data.(bool); ok && hasUpdate {
				extras["updateAvailable"] = true
			}
		}
	case <-ctx.Done():
		// Continue without update status
	}

	// Determine final status and message
	if len(allWarnings) > 0 {
		healthStatus = "warning"
		healthMessage = strings.Join(allWarnings, "\n\n")
	} else {
		healthStatus = "online"
		healthMessage = "Healthy"
	}

	return s.CreateHealthResponse(ihc.startTime, healthStatus, healthMessage, extras), http.StatusOK
}

func formatWarning(issue HealthResponse) string {
	// Check for known warning patterns
	message := issue.Message
	for _, warning := range knownWarnings {
		if strings.Contains(message, warning.Pattern) {
			return fmt.Sprintf("[%s] %s", warning.Category, message)
		}
	}

	// If no specific pattern is matched, use the source if available
	if issue.Source != "" &&
		issue.Source != "IndexerLongTermStatusCheck" &&
		issue.Source != "NotificationStatusCheck" {
		return fmt.Sprintf("[%s] %s", issue.Source, message)
	}

	return message
}
