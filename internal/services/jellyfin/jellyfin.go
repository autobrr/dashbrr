// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	jellyfinVersionCacheTTL = 1 * time.Hour
)

type ErrJellyfin struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrJellyfin) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("jellyfin %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("jellyfin %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("jellyfin %s", e.Op)
}

func (e *ErrJellyfin) Unwrap() error {
	return e.Err
}

type JellyfinService struct {
	core.ServiceCore
}

func init() {
	models.NewJellyfinService = NewJellyfinService
}

func NewJellyfinService() models.ServiceHealthChecker {
	service := &JellyfinService{}
	service.Type = "jellyfin"
	service.DisplayName = "Jellyfin"
	service.Description = "Monitor and manage your Jellyfin server"
	service.DefaultURL = "http://localhost:8096"
	service.HealthEndpoint = "/System/Info"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *JellyfinService) GetHealthEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/System/Info", strings.TrimRight(baseURL, "/"))
}

func (s *JellyfinService) getHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Accept":       "application/json",
		"X-Emby-Token": apiKey,
	}
}

func (s *JellyfinService) buildURL(baseURL, path string, query url.Values) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if trimmed == "" {
		return "", errors.New("URL is required")
	}

	endpoint, err := url.Parse(trimmed + path)
	if err != nil {
		return "", err
	}

	if query != nil {
		existing := endpoint.Query()
		for key, values := range query {
			for _, value := range values {
				existing.Add(key, value)
			}
		}
		endpoint.RawQuery = existing.Encode()
	}

	return endpoint.String(), nil
}

func (s *JellyfinService) getJSON(ctx context.Context, op, baseURL, apiKey, path string, query url.Values, out any) error {
	if strings.TrimSpace(baseURL) == "" {
		return &ErrJellyfin{Op: op, Err: errors.New("URL is required")}
	}
	if strings.TrimSpace(apiKey) == "" {
		return &ErrJellyfin{Op: op, Err: errors.New("API key is required")}
	}

	endpoint, err := s.buildURL(baseURL, path, query)
	if err != nil {
		return &ErrJellyfin{Op: op, Err: fmt.Errorf("failed to build request URL: %w", err)}
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, s.getHeaders(apiKey), nil)
	if err != nil {
		return &ErrJellyfin{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrJellyfin{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrJellyfin{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrJellyfin{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return nil
}

func (s *JellyfinService) GetSystemInfo(ctx context.Context, baseURL, apiKey string) (types.JellyfinSystemInfo, error) {
	var info types.JellyfinSystemInfo
	if err := s.getJSON(ctx, "get_system_info", baseURL, apiKey, "/System/Info", nil, &info); err != nil {
		return types.JellyfinSystemInfo{}, err
	}
	return info, nil
}

func (s *JellyfinService) GetVersion(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := s.GetVersionFromCache(ctx, baseURL); version != "" && version != "true" {
		return version, nil
	}

	info, err := s.GetSystemInfo(ctx, baseURL, apiKey)
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(info.Version)
	if version == "" {
		return "", &ErrJellyfin{Op: "get_version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, jellyfinVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache Jellyfin version")
	}

	return version, nil
}

func (s *JellyfinService) GetSessions(ctx context.Context, baseURL, apiKey string) ([]types.JellyfinSession, error) {
	var sessions []types.JellyfinSession
	query := url.Values{}
	query.Set("ActiveWithinSeconds", "300")
	if err := s.getJSON(ctx, "get_sessions", baseURL, apiKey, "/Sessions", query, &sessions); err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []types.JellyfinSession{}
	}

	active := make([]types.JellyfinSession, 0, len(sessions))
	for _, session := range sessions {
		if session.NowPlayingItem == nil {
			continue
		}
		if strings.TrimSpace(session.NowPlayingItem.Name) == "" && strings.TrimSpace(session.NowPlayingItem.SeriesName) == "" {
			continue
		}
		active = append(active, session)
	}

	return active, nil
}

func (s *JellyfinService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.JellyfinSummaryResponse, error) {
	var (
		systemInfo  types.JellyfinSystemInfo
		systemErr   error
		sessions    []types.JellyfinSession
		sessionsErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		systemInfo, systemErr = s.GetSystemInfo(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		sessions, sessionsErr = s.GetSessions(ctx, baseURL, apiKey)
	}()

	wg.Wait()

	if systemErr != nil && sessionsErr != nil {
		return types.JellyfinSummaryResponse{}, fmt.Errorf(
			"all jellyfin summary requests failed: system: %v, sessions: %v",
			systemErr,
			sessionsErr,
		)
	}

	if sessions == nil {
		sessions = []types.JellyfinSession{}
	}

	return types.JellyfinSummaryResponse{
		System:   systemInfo,
		Sessions: sessions,
	}, nil
}

func (s *JellyfinService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}
	if strings.TrimSpace(apiKey) == "" {
		return s.CreateHealthResponse(start, "error", "API key is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	info, err := s.GetSystemInfo(healthCtx, baseURL, apiKey)
	if err != nil {
		return s.CreateHealthResponse(start, "offline", fmt.Sprintf("Failed to connect: %v", err), map[string]any{
			"responseTime": time.Since(start).Milliseconds(),
		}), http.StatusOK
	}

	version, versionErr := s.GetVersion(healthCtx, baseURL, apiKey)
	if versionErr != nil {
		version = strings.TrimSpace(info.Version)
	}

	message := "Healthy"
	if strings.TrimSpace(info.ServerName) != "" {
		message = fmt.Sprintf("Healthy - %s", strings.TrimSpace(info.ServerName))
	}

	extras := map[string]any{
		"responseTime":    time.Since(start).Milliseconds(),
		"updateAvailable": s.GetUpdateStatusFromCache(ctx, baseURL),
	}
	if strings.TrimSpace(version) != "" {
		extras["version"] = strings.TrimSpace(version)
	}

	return s.CreateHealthResponse(start, "online", message, extras), http.StatusOK
}
