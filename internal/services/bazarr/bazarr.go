// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package bazarr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/rs/zerolog/log"
)

const (
	bazarrVersionCacheTTL = 1 * time.Hour
)

type ErrBazarr struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrBazarr) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("bazarr %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("bazarr %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("bazarr %s", e.Op)
}

func (e *ErrBazarr) Unwrap() error {
	return e.Err
}

type BazarrService struct {
	core.ServiceCore
}

func init() {
	models.NewBazarrService = NewBazarrService
}

func NewBazarrService() models.ServiceHealthChecker {
	service := &BazarrService{}
	service.Type = "bazarr"
	service.DisplayName = "Bazarr"
	service.Description = "Monitor and manage your Bazarr instance"
	service.DefaultURL = "http://localhost:6767"
	service.HealthEndpoint = "/api/system/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *BazarrService) headers(apiKey string) map[string]string {
	return map[string]string{
		"X-API-KEY": apiKey,
	}
}

func (s *BazarrService) endpoint(baseURL, path string) string {
	return fmt.Sprintf("%s%s", strings.TrimRight(baseURL, "/"), path)
}

func (s *BazarrService) getJSON(ctx context.Context, op, baseURL, path, apiKey string, out any) error {
	if strings.TrimSpace(baseURL) == "" {
		return &ErrBazarr{Op: op, Err: errors.New("URL is required")}
	}
	if strings.TrimSpace(apiKey) == "" {
		return &ErrBazarr{Op: op, Err: errors.New("API key is required")}
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, s.endpoint(baseURL, path), s.headers(apiKey), nil)
	if err != nil {
		return &ErrBazarr{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrBazarr{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrBazarr{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrBazarr{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return nil
}

func (s *BazarrService) GetHealthEndpoint(baseURL string) string {
	return s.endpoint(baseURL, "/api/system/health")
}

func (s *BazarrService) GetSystemStatus(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := s.GetVersionFromCache(ctx, baseURL); version != "" && version != "true" {
		return version, nil
	}

	var status types.BazarrSystemStatusEnvelope
	if err := s.getJSON(ctx, "get_system_status", baseURL, "/api/system/status", apiKey, &status); err != nil {
		return "", err
	}

	version := strings.TrimSpace(status.Data.BazarrVersion)
	if version == "" {
		return "", &ErrBazarr{Op: "get_system_status", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, bazarrVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache Bazarr version")
	}

	return version, nil
}

func (s *BazarrService) GetBadges(ctx context.Context, baseURL, apiKey string) (types.BazarrBadges, error) {
	var badges types.BazarrBadges
	if err := s.getJSON(ctx, "get_badges", baseURL, "/api/badges", apiKey, &badges); err != nil {
		return types.BazarrBadges{}, err
	}
	return badges, nil
}

func (s *BazarrService) GetProviders(ctx context.Context, baseURL, apiKey string) ([]types.BazarrProviderStatus, error) {
	var providers types.BazarrProvidersEnvelope
	if err := s.getJSON(ctx, "get_providers", baseURL, "/api/providers", apiKey, &providers); err != nil {
		return nil, err
	}
	if providers.Data == nil {
		return []types.BazarrProviderStatus{}, nil
	}
	return providers.Data, nil
}

func (s *BazarrService) GetHealthIssues(ctx context.Context, baseURL, apiKey string) ([]types.BazarrHealthIssue, error) {
	var health types.BazarrHealthIssuesEnvelope
	if err := s.getJSON(ctx, "get_health_issues", baseURL, "/api/system/health", apiKey, &health); err != nil {
		return nil, err
	}
	if health.Data == nil {
		return []types.BazarrHealthIssue{}, nil
	}
	return health.Data, nil
}

func (s *BazarrService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.BazarrSummaryResponse, error) {
	var (
		summary         types.BazarrSummaryResponse
		badgesErr       error
		providersErr    error
		healthIssuesErr error
	)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		summary.Badges, badgesErr = s.GetBadges(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		summary.Providers, providersErr = s.GetProviders(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		summary.HealthIssues, healthIssuesErr = s.GetHealthIssues(ctx, baseURL, apiKey)
	}()

	wg.Wait()

	if badgesErr != nil && providersErr != nil && healthIssuesErr != nil {
		return types.BazarrSummaryResponse{}, fmt.Errorf(
			"all bazarr summary requests failed: badges: %v, providers: %v, health: %v",
			badgesErr,
			providersErr,
			healthIssuesErr,
		)
	}

	if summary.Providers == nil {
		summary.Providers = []types.BazarrProviderStatus{}
	}
	if summary.HealthIssues == nil {
		summary.HealthIssues = []types.BazarrHealthIssue{}
	}

	return summary, nil
}

func (s *BazarrService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}
	if strings.TrimSpace(apiKey) == "" {
		return s.CreateHealthResponse(start, "error", "API key is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	var (
		version    string
		issues     []types.BazarrHealthIssue
		versionErr error
		issuesErr  error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		version, versionErr = s.GetSystemStatus(healthCtx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		issues, issuesErr = s.GetHealthIssues(healthCtx, baseURL, apiKey)
	}()

	wg.Wait()

	if issuesErr != nil {
		return s.CreateHealthResponse(start, "error", fmt.Sprintf("Health check failed: %v", issuesErr)), http.StatusOK
	}

	extras := map[string]any{
		"responseTime": time.Since(start).Milliseconds(),
	}
	if versionErr == nil && version != "" {
		extras["version"] = version
	}

	if len(issues) == 0 {
		return s.CreateHealthResponse(start, "online", "Healthy", extras), http.StatusOK
	}

	return s.CreateHealthResponse(start, "warning", formatHealthIssues(issues), extras), http.StatusOK
}

func formatHealthIssues(issues []types.BazarrHealthIssue) string {
	if len(issues) == 0 {
		return "Healthy"
	}

	seen := make(map[string]struct{}, len(issues))
	messages := make([]string, 0, len(issues))

	for _, issue := range issues {
		object := strings.TrimSpace(issue.Object)
		description := strings.TrimSpace(issue.Issue)

		var msg string
		switch {
		case object == "" && description == "":
			continue
		case object == "":
			msg = description
		case description == "":
			msg = object
		default:
			msg = fmt.Sprintf("[%s] %s", object, description)
		}

		key := strings.ToLower(msg)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		return "Healthy"
	}

	const maxWarnings = 6
	if len(messages) <= maxWarnings {
		return strings.Join(messages, "\n\n")
	}

	remaining := len(messages) - maxWarnings
	return strings.Join(append(messages[:maxWarnings], fmt.Sprintf("... and %d more", remaining)), "\n\n")
}
