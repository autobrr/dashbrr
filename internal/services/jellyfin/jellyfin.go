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
	Service  string
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrJellyfin) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("%s %s: server returned %s (%d)", e.Service, e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s %s: %v", e.Service, e.Op, e.Err)
	}
	return fmt.Sprintf("%s %s", e.Service, e.Op)
}

func (e *ErrJellyfin) Unwrap() error {
	return e.Err
}

// JellyfinService is the client for Jellyfin and for Emby. The two servers
// share the API that dashbrr reads. The fields below hold the differences.
type JellyfinService struct {
	core.ServiceCore
	// authHeader gives the name and the value of the header that sends the API key.
	authHeader func(apiKey string) (string, string)
	// pathPrefix goes between the base URL and each API path.
	pathPrefix string
	// sessionQuery goes on each /Sessions request.
	sessionQuery url.Values
}

func init() {
	models.NewJellyfinService = NewJellyfinService
	models.NewEmbyService = NewEmbyService
}

func NewJellyfinService() models.ServiceHealthChecker {
	service := &JellyfinService{
		Type:           "jellyfin",
		DisplayName:    "Jellyfin",
		Description:    "Monitor and manage your Jellyfin server",
		DefaultURL:     "http://localhost:8096",
		HealthEndpoint: "/System/Info",
		// Jellyfin 12 disables the legacy token headers, X-Emby-Token included.
		authHeader: func(apiKey string) (string, string) {
			return "Authorization", fmt.Sprintf("MediaBrowser Token=%q", apiKey)
		},
		sessionQuery: url.Values{"ActiveWithinSeconds": {"300"}},
	}
	service.SetTimeout(core.DefaultTimeout)
	return service
}

// NewEmbyService returns the client for Emby. Emby documents the X-Emby-Token
// header and the /emby path prefix, and has no ActiveWithinSeconds query.
// See docs/research/emby-api.md.
func NewEmbyService() models.ServiceHealthChecker {
	service := &JellyfinService{
		Type:           "emby",
		DisplayName:    "Emby",
		Description:    "Monitor and manage your Emby server",
		DefaultURL:     "http://localhost:8096",
		HealthEndpoint: "/emby/System/Info",
		authHeader: func(apiKey string) (string, string) {
			return "X-Emby-Token", apiKey
		},
		pathPrefix: "/emby",
	}
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *JellyfinService) GetHealthEndpoint(baseURL string) string {
	return s.baseURL(baseURL) + "/System/Info"
}

func (s *JellyfinService) getHeaders(apiKey string) map[string]string {
	name, value := s.authHeader(apiKey)
	return map[string]string{
		"Accept": "application/json",
		name:     value,
	}
}

// baseURL returns the URL that API paths go after. A user can copy an Emby
// URL that already ends in the prefix, so one copy of the prefix is removed
// before it is added again.
func (s *JellyfinService) baseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if s.pathPrefix == "" {
		return trimmed
	}
	return strings.TrimSuffix(trimmed, s.pathPrefix) + s.pathPrefix
}

func (s *JellyfinService) buildURL(baseURL, path string, query url.Values) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", errors.New("URL is required")
	}

	endpoint, err := url.Parse(s.baseURL(baseURL) + path)
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
		return &ErrJellyfin{Service: s.Type, Op: op, Err: errors.New("URL is required")}
	}
	if strings.TrimSpace(apiKey) == "" {
		return &ErrJellyfin{Service: s.Type, Op: op, Err: errors.New("API key is required")}
	}

	endpoint, err := s.buildURL(baseURL, path, query)
	if err != nil {
		return &ErrJellyfin{Service: s.Type, Op: op, Err: fmt.Errorf("failed to build request URL: %w", err)}
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, s.getHeaders(apiKey), nil)
	if err != nil {
		return &ErrJellyfin{Service: s.Type, Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrJellyfin{Service: s.Type, Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrJellyfin{Service: s.Type, Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrJellyfin{Service: s.Type, Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
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
		return "", &ErrJellyfin{Service: s.Type, Op: "get_version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, jellyfinVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", core.RedactURL(baseURL)).Str("version", version).Msgf("Failed to cache %s version", s.DisplayName)
	}

	return version, nil
}

func (s *JellyfinService) GetSessions(ctx context.Context, baseURL, apiKey string) ([]types.JellyfinSession, error) {
	var sessions []types.JellyfinSession
	if err := s.getJSON(ctx, "get_sessions", baseURL, apiKey, "/Sessions", s.sessionQuery, &sessions); err != nil {
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
			"all %s summary requests failed: system: %v, sessions: %v",
			s.Type,
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
