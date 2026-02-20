// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package traefik

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	traefikVersionCacheTTL = 1 * time.Hour
	traefikVersionPath     = "/api/version"
	traefikOverviewPath    = "/api/overview"
	traefikRoutersPath     = "/api/http/routers"
	maxIssueRouters        = 25
)

type ErrTraefik struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrTraefik) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("traefik %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("traefik %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("traefik %s", e.Op)
}

func (e *ErrTraefik) Unwrap() error {
	return e.Err
}

type TraefikService struct {
	core.ServiceCore
}

func init() {
	models.NewTraefikService = NewTraefikService
}

func NewTraefikService() models.ServiceHealthChecker {
	service := &TraefikService{}
	service.Type = "traefik"
	service.DisplayName = "Traefik"
	service.Description = "Monitor Traefik routers and runtime health"
	service.DefaultURL = "http://localhost:8080"
	service.HealthEndpoint = traefikVersionPath
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *TraefikService) GetHealthEndpoint(baseURL string) string {
	endpoint, err := s.buildEndpoint(baseURL, traefikVersionPath, nil)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + traefikVersionPath
	}
	return endpoint
}

func (s *TraefikService) buildEndpoint(baseURL, path string, query url.Values) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if trimmed == "" {
		return "", errors.New("URL is required")
	}

	endpoint, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	if query != nil {
		endpoint.RawQuery = query.Encode()
	} else {
		endpoint.RawQuery = ""
	}
	return endpoint.String(), nil
}

func authHeaderFromAPIKey(apiKey string) string {
	token := strings.TrimSpace(apiKey)
	if token == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}

	if strings.Contains(token, ":") {
		encoded := base64.StdEncoding.EncodeToString([]byte(token))
		return "Basic " + encoded
	}

	return "Bearer " + token
}

func (s *TraefikService) getJSON(
	ctx context.Context,
	op,
	baseURL,
	apiKey,
	path string,
	query url.Values,
	out any,
) error {
	endpoint, err := s.buildEndpoint(baseURL, path, query)
	if err != nil {
		return &ErrTraefik{Op: op, Err: err}
	}

	headers := map[string]string{
		"Accept": "application/json",
	}
	if authHeader := authHeaderFromAPIKey(apiKey); authHeader != "" {
		headers["Authorization"] = authHeader
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrTraefik{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return nil
}

func (s *TraefikService) GetVersion(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := strings.TrimSpace(s.GetVersionFromCache(ctx, baseURL)); version != "" && version != "true" {
		return version, nil
	}

	var payload types.TraefikVersionResponse
	if err := s.getJSON(ctx, "get_version", baseURL, apiKey, traefikVersionPath, nil, &payload); err != nil {
		return "", err
	}

	version := strings.TrimSpace(payload.Version)
	if version == "" {
		return "", &ErrTraefik{Op: "get_version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, traefikVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("failed to cache Traefik version")
	}

	return version, nil
}

func (s *TraefikService) GetOverview(ctx context.Context, baseURL, apiKey string) (types.TraefikOverviewResponse, error) {
	var payload types.TraefikOverviewResponse
	if err := s.getJSON(ctx, "get_overview", baseURL, apiKey, traefikOverviewPath, nil, &payload); err != nil {
		return types.TraefikOverviewResponse{}, err
	}
	return payload, nil
}

func (s *TraefikService) GetRoutersByStatus(
	ctx context.Context,
	baseURL,
	apiKey,
	status string,
) ([]types.TraefikRouter, error) {
	query := url.Values{}
	query.Set("status", status)
	query.Set("per_page", "100")

	var payload []types.TraefikRouter
	if err := s.getJSON(ctx, "get_http_routers", baseURL, apiKey, traefikRoutersPath, query, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = []types.TraefikRouter{}
	}
	return payload, nil
}

func mergeIssueRouters(warningRouters, disabledRouters []types.TraefikRouter) []types.TraefikRouter {
	merged := make([]types.TraefikRouter, 0, len(warningRouters)+len(disabledRouters))
	seen := make(map[string]struct{}, len(warningRouters)+len(disabledRouters))

	appendUnique := func(routers []types.TraefikRouter) {
		for _, router := range routers {
			key := strings.TrimSpace(router.Name) + "@" + strings.TrimSpace(router.Provider)
			if key == "@" {
				key = strings.TrimSpace(router.Rule) + "|" + strings.TrimSpace(router.Service)
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, router)
		}
	}

	appendUnique(disabledRouters)
	appendUnique(warningRouters)

	sort.SliceStable(merged, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(merged[i].Status))
		lj := strings.ToLower(strings.TrimSpace(merged[j].Status))
		if li != lj {
			return li < lj // disabled before warning
		}
		return strings.ToLower(merged[i].Name) < strings.ToLower(merged[j].Name)
	})

	if len(merged) > maxIssueRouters {
		return merged[:maxIssueRouters]
	}

	return merged
}

func (s *TraefikService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.TraefikSummaryResponse, error) {
	var (
		overview        types.TraefikOverviewResponse
		overviewErr     error
		warningRouters  []types.TraefikRouter
		warningErr      error
		disabledRouters []types.TraefikRouter
		disabledErr     error
	)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		overview, overviewErr = s.GetOverview(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		warningRouters, warningErr = s.GetRoutersByStatus(ctx, baseURL, apiKey, "warning")
	}()

	go func() {
		defer wg.Done()
		disabledRouters, disabledErr = s.GetRoutersByStatus(ctx, baseURL, apiKey, "disabled")
	}()

	wg.Wait()

	if overviewErr != nil {
		return types.TraefikSummaryResponse{}, overviewErr
	}
	if warningErr != nil {
		log.Debug().Err(warningErr).Msg("traefik warning routers fetch failed")
	}
	if disabledErr != nil {
		log.Debug().Err(disabledErr).Msg("traefik disabled routers fetch failed")
	}

	return types.TraefikSummaryResponse{
		Overview:     overview,
		IssueRouters: mergeIssueRouters(warningRouters, disabledRouters),
	}, nil
}

func (s *TraefikService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	version, err := s.GetVersion(healthCtx, baseURL, apiKey)
	if err != nil {
		return s.CreateHealthResponse(start, "offline", fmt.Sprintf("Failed to connect: %v", err), map[string]interface{}{
			"responseTime": time.Since(start).Milliseconds(),
		}), http.StatusOK
	}

	return s.CreateHealthResponse(start, "online", "Healthy", map[string]interface{}{
		"responseTime": time.Since(start).Milliseconds(),
		"version":      version,
	}), http.StatusOK
}
