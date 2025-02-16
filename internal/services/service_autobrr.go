// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// AutobrrClient represents an Autobrr service client
type AutobrrClient struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

// AutobrrHealthCheckResponse represents the response from a health check
type AutobrrHealthCheckResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// NewAutobrrClient creates a new Autobrr service client
func NewAutobrrClient(baseURL, apiKey string) *AutobrrClient {
	return &AutobrrClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HealthCheck performs a health check on the Autobrr service
func (c *AutobrrClient) HealthCheck() (*AutobrrHealthCheckResponse, error) {
	// For now, return a mock health check response
	// In a real implementation, you'd make an actual HTTP request
	return &AutobrrHealthCheckResponse{
		Status:  "OK",
		Version: "1.0.0", // This would be dynamically retrieved in a real implementation
	}, nil
}

type AutobrrService struct {
	ServiceCore
}

func NewAutobrrService(db *database.DB, cache cache.Store, config *domain.ServiceConfiguration) *AutobrrService {
	log.Trace().Msg("initializing new Autobrr instance")

	service := &AutobrrService{
		//ServiceCore: ServiceCore{
		//	Type: domain.ServiceTypeAutobrr,
		//
		//},
	}
	service.Type = domain.ServiceTypeAutobrr
	service.DisplayName = config.DisplayName
	service.Description = "Monitor and manage your Autobrr instance"
	service.DefaultURL = "http://localhost:7474"
	service.HealthEndpoint = "/api/healthz/liveness"
	service.URL = config.URL
	service.ApiKey = config.APIKey
	service.InstanceID = config.InstanceID
	service.SetTimeout(DefaultTimeout)
	service.SetDB(db)
	service.SetCache(cache)
	return service
}

func (s *AutobrrService) getEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s%s", baseURL, path)
}

func (s *AutobrrService) GetReleases(ctx context.Context, url, apiKey string) (domain.ReleasesResponse, error) {
	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(url, "/api/release"), apiKey, s.headers())
	if err != nil {
		return domain.ReleasesResponse{}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.ReleasesResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return domain.ReleasesResponse{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var releases domain.ReleasesResponse
	if err := json.Unmarshal(body, &releases); err != nil {
		return domain.ReleasesResponse{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return releases, nil
}

func (s *AutobrrService) GetReleaseStats(ctx context.Context) (domain.AutobrrStats, error) {
	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(s.URL, "/api/release/stats"), s.ApiKey, s.headers())
	if err != nil {
		return domain.AutobrrStats{}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.AutobrrStats{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// TODO do not use
	body, err := s.ReadBody(resp)
	if err != nil {
		return domain.AutobrrStats{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var stats domain.AutobrrStats
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()

	if err := decoder.Decode(&stats); err != nil {
		return domain.AutobrrStats{}, fmt.Errorf("failed to decode response: %v, body: %s", err, string(body))
	}

	return stats, nil
}

func (s *AutobrrService) GetIRCStatusFromCache(ctx context.Context) string {
	var status string
	if err := s.GetDataFromCache(ctx, "autobrr"+":irc:"+s.InstanceID, status); err != nil {
		return ""
	}

	return status
}

func (s *AutobrrService) CacheIRCStatus(ctx context.Context, status string) error {
	return s.CacheData(ctx, "autobrr"+":irc:"+s.InstanceID, status, 5*time.Minute)
}

func (s *AutobrrService) ValidConfig() error {
	if s.URL == "" {
		return fmt.Errorf("service not configured: missing URL")
	}

	if s.ApiKey == "" {
		return fmt.Errorf("service not configured: missing API key")
	}

	return nil
}

func (s *AutobrrService) GetIRCStatus(ctx context.Context) ([]domain.IRCStatus, error) {
	// Check cache first
	if cached := s.GetIRCStatusFromCache(ctx); cached != "" {
		var status []domain.IRCStatus
		if err := json.Unmarshal([]byte(cached), &status); err == nil {
			return status, nil
		}
	}

	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(s.URL, "/api/irc"), s.ApiKey, s.headers())
	if err != nil {
		return []domain.IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []domain.IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return []domain.IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("failed to read response body: %v", err)
	}

	// Try to decode as array first
	var allStatus []domain.IRCStatus
	if err := json.Unmarshal(body, &allStatus); err == nil {
		var unhealthyStatus []domain.IRCStatus
		for _, status := range allStatus {
			if !status.Healthy && status.Enabled {
				unhealthyStatus = append(unhealthyStatus, status)
			}
		}
		// Cache the result
		if cached, err := json.Marshal(unhealthyStatus); err == nil {
			if err := s.CacheIRCStatus(ctx, string(cached)); err != nil {
				fmt.Printf("Failed to cache IRC status: %v\n", err)
			}
		}
		return unhealthyStatus, nil
	}

	// If array decode fails, try to decode as single object
	var singleStatus domain.IRCStatus
	if err := json.Unmarshal(body, &singleStatus); err == nil {
		// Only return if unhealthy AND enabled
		if !singleStatus.Healthy && singleStatus.Enabled {
			status := []domain.IRCStatus{singleStatus}
			// Cache the result
			if cached, err := json.Marshal(status); err == nil {
				if err := s.CacheIRCStatus(ctx, string(cached)); err != nil {
					fmt.Printf("Failed to cache IRC status: %v\n", err)
				}
			}
			return status, nil
		}
		// Cache empty result
		// TODO keep?
		if err := s.CacheIRCStatus(ctx, "[]"); err != nil {
			fmt.Printf("Failed to cache IRC status: %v\n", err)
		}
		return []domain.IRCStatus{}, nil
	}

	return []domain.IRCStatus{{Name: "IRC", Healthy: false}}, fmt.Errorf("failed to decode response: %s", string(body))
}

func (s *AutobrrService) GetVersion(ctx context.Context) (string, error) {
	// Check cache first, ensuring we don't return "true" as a version
	if version := s.GetVersionFromCache(s.URL); version != "" && version != "true" {
		return version, nil
	}

	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(s.URL, "/api/config"), s.ApiKey, s.headers())
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

	var versionData domain.VersionResponse
	if err := json.Unmarshal(body, &versionData); err != nil {
		return "", err
	}

	// Cache version for 2 hours to align with update check
	if err := s.CacheVersion(nil, s.URL, versionData.Version, 2*time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return versionData.Version, nil
}

func (s *AutobrrService) GetUpdateFromCache(ctx context.Context) string {
	var update string
	s.GetDataFromCache(ctx, "autobrr"+":update:"+s.InstanceID, update)

	return update
}

func (s *AutobrrService) CacheUpdate(ctx context.Context, status string, ttl time.Duration) error {
	return s.CacheData(ctx, "autobrr"+":update:"+s.InstanceID, status, ttl)
}

func (s *AutobrrService) CheckUpdate(ctx context.Context) (bool, error) {
	// Check cache first
	if status := s.GetUpdateFromCache(ctx); status != "" {
		return status == "true", nil
	}

	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(s.URL, "/api/updates/latest"), s.ApiKey, s.headers())
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
	if err := s.CacheUpdate(ctx, status, 2*time.Hour); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache update status: %v\n", err)
	}

	return hasUpdate, nil
}

func (s *AutobrrService) CheckHealth(ctx context.Context, _, _ string) (*domain.ServiceHealth, int) {
	log.Trace().Str("url", s.URL).Msg("Checking autobrr service health")

	//startTime := time.Now()

	res := &domain.ServiceHealth{
		Status:          "online",
		Message:         "autobrr is running",
		ResponseTime:    0,
		LastChecked:     time.Time{},
		Version:         "",
		UpdateAvailable: false,
		ServiceID:       s.InstanceID,
		Services: &domain.ServiceHealthCheckResponse{
			Autobrr: domain.ServiceHealthResponseAutobrr{
				Stats: domain.AutobrrStats{},
				IRC: domain.AutobrrIRC{
					Healthy:  true,
					Networks: make([]domain.IRCStatus, 0),
				},
			},
		},
	}

	wg := sync.WaitGroup{}

	//// Create a context with timeout for the entire health check
	//ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	//defer cancel()

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		if err := s.liveness(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to perform liveness check")
		}
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		version, err := s.GetVersion(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get version")
			return
		}
		res.Version = version
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		hasUpdate, err := s.CheckUpdate(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check for update")
			return
		}
		res.UpdateAvailable = hasUpdate
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		// Get release stats
		stats, err := s.GetReleaseStats(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get release stats")
			// Continue without stats, don't fail the health check
		}

		res.Services.Autobrr.Stats = stats
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		// Get IRC status
		ircStatus, err := s.GetIRCStatus(ctx)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get IRC status")
			res.Message = "Autobrr is running but IRC status check failed: " + err.Error()
			res.Status = "warning"
		}

		// Check if any IRC connection is unhealthy
		for _, status := range ircStatus {
			if !status.Healthy {
				res.Services.Autobrr.IRC.Healthy = status.Healthy
				res.Message = "Autobrr is running but reports unhealthy IRC connections"
				res.Status = "warning"
				break
			}
		}
	}(&wg)

	wg.Wait()

	return res, http.StatusOK
}

// liveness check
func (s *AutobrrService) liveness(ctx context.Context) error {
	// Perform health check
	resp, err := s.MakeRequestWithContext(ctx, s.getEndpoint(s.URL, "/api/healthz/liveness"), s.ApiKey, s.headers())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return err
	}

	trimmedBody := strings.TrimSpace(string(body))
	trimmedBody = strings.Trim(trimmedBody, "\"")

	if trimmedBody != "healthy" && trimmedBody != "OK" {
		return errors.Errorf("unhealthy: %s", trimmedBody)
	}
	return nil
}

func (s *AutobrrService) headers() map[string]string {
	// Perform health check
	headers := map[string]string{
		"auth_header": "X-Api-Token",
		"auth_value":  s.ApiKey,
	}
	return headers
}
