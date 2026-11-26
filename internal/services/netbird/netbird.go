// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package netbird

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	version "github.com/hashicorp/go-version"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// DefaultURL is the NetBird Cloud API. A self-hosting user replaces it with the
// address of their own management server.
const DefaultURL = "https://api.netbird.io"

// tokenPrefix marks a NetBird personal access token.
const tokenPrefix = "nbp_"

// allGroup is the group that every peer belongs to. It carries no information,
// so it stays out of the peer list.
const allGroup = "All"

const (
	// versionTTL caches the answer of /api/instance/version for one hour.
	versionTTL = time.Hour
	// versionMissTTL remembers a server that answered "no such endpoint" for a
	// while, so a NetBird without it is not asked again on every poll.
	versionMissTTL = 5 * time.Minute
	// versionTimeout limits the optional version call. A version endpoint that
	// does not answer must not delay the health check or the peer list.
	versionTimeout = 3 * time.Second
	// versionBudgetMargin is the room a caller must still have beyond the version
	// timeout, so the answer it already holds can be written before its deadline.
	versionBudgetMargin = time.Second
	// versionMissMarker stands in the cache slot for "this server has no usable
	// version endpoint".
	versionMissMarker = "unavailable"
)

type Service struct {
	core.ServiceCore
}

// Peer is what the dashboard shows for one NetBird peer.
type Peer struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	IP              string   `json:"ip"`
	Online          bool     `json:"online"`
	LastSeen        string   `json:"lastSeen"`
	OS              string   `json:"os"`
	Version         string   `json:"version"`
	UpdateAvailable bool     `json:"updateAvailable"`
	Groups          []string `json:"groups"`
	CityName        string   `json:"cityName"`
	CountryCode     string   `json:"countryCode"`
	LoginExpired    bool     `json:"loginExpired"`
}

type apiPeer struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Connected    bool   `json:"connected"`
	LastSeen     string `json:"last_seen"`
	OS           string `json:"os"`
	Version      string `json:"version"`
	LoginExpired bool   `json:"login_expired"`
	CityName     string `json:"city_name"`
	CountryCode  string `json:"country_code"`
	Groups       []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"groups"`
}

type versionInfo struct {
	ManagementCurrentVersion   string `json:"management_current_version"`
	ManagementAvailableVersion string `json:"management_available_version"`
	ManagementUpdateAvailable  bool   `json:"management_update_available"`
}

func NewNetBirdService() models.ServiceHealthChecker {
	service := &Service{}
	service.Type = "netbird"
	service.DisplayName = "NetBird"
	service.Description = "Monitor the peers of your NetBird network"
	service.DefaultURL = DefaultURL
	service.HealthEndpoint = "/api/peers"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func peerURL(baseURL string) (string, error) {
	return url.JoinPath(baseURL, "api", "peers")
}

func versionURL(baseURL string) (string, error) {
	return url.JoinPath(baseURL, "api", "instance", "version")
}

func (s *Service) fetchPeers(ctx context.Context, baseURL, apiKey string) ([]apiPeer, time.Duration, error) {
	startTime := time.Now()

	if !strings.HasPrefix(apiKey, tokenPrefix) {
		return nil, 0, fmt.Errorf("invalid token format. Must start with '%s'", tokenPrefix)
	}

	headers := map[string]string{
		"Authorization": "Token " + apiKey,
	}

	endpoint, err := peerURL(baseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid URL: %w", err)
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, responseTime, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, responseTime, fmt.Errorf("API request failed (Status %d): %s", resp.StatusCode, string(body))
	}

	var peers []apiPeer
	if err := json.Unmarshal(body, &peers); err != nil {
		return nil, responseTime, fmt.Errorf("failed to decode response: %w", err)
	}

	return peers, responseTime, nil
}

// fetchVersion asks the management server which version is current and which is
// the newest release. NetBird clients and the management server share version
// numbers, so one answer covers the peers and the server. The endpoint is
// optional: when it fails, the zero versionInfo means no badges and no error.
func (s *Service) fetchVersion(ctx context.Context, baseURL, apiKey string) versionInfo {
	var info versionInfo
	if cached := s.GetVersionFromCache(ctx, baseURL); cached != "" {
		if cached != versionMissMarker {
			_ = json.Unmarshal([]byte(cached), &info)
		}
		return info
	}

	// A caller with a deadline of its own, such as the health handler, must not
	// lose the answer it already has because this optional request eats what is
	// left of the deadline, so a slow peer call skips the version request.
	if !versionFitsIn(ctx) {
		return info
	}

	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()

	info, err := s.requestVersion(ctx, baseURL, apiKey)
	if err != nil {
		log.Debug().Err(err).Str("url", core.RedactURL(baseURL)).Msg("NetBird version check failed, showing no update badges")
	}
	return info
}

// versionFitsIn reports whether the caller's deadline leaves room for the
// optional version call plus a margin. Without a deadline there is always room.
func versionFitsIn(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return true
	}

	return time.Until(deadline) > versionTimeout+versionBudgetMargin
}

func (s *Service) requestVersion(ctx context.Context, baseURL, apiKey string) (versionInfo, error) {
	headers := map[string]string{
		"Authorization": "Token " + apiKey,
	}

	endpoint, err := versionURL(baseURL)
	if err != nil {
		return versionInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return versionInfo{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// The check has to happen before ReadBody, which turns any non-200 into an error.
	if resp.StatusCode == http.StatusNotFound {
		_ = s.CacheVersion(ctx, baseURL, versionMissMarker, versionMissTTL)
		return versionInfo{}, errors.New("server has no version endpoint")
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return versionInfo{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return versionInfo{}, fmt.Errorf("API request failed (Status %d): %s", resp.StatusCode, string(body))
	}

	var info versionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return versionInfo{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if info.ManagementCurrentVersion != "" {
		if encoded, err := json.Marshal(info); err == nil {
			_ = s.CacheVersion(ctx, baseURL, string(encoded), versionTTL)
		}
	}

	return info, nil
}

// needsUpdate reports whether a peer version is older than the newest release.
// A version that the parser cannot read, for example "development", gets no badge.
func needsUpdate(peerVersion, availableVersion string) bool {
	if peerVersion == "" || availableVersion == "" {
		return false
	}

	current, err := version.NewVersion(peerVersion)
	if err != nil {
		return false
	}

	newest, err := version.NewVersion(availableVersion)
	if err != nil {
		return false
	}

	return current.LessThan(newest)
}

func groupNames(peer apiPeer) []string {
	names := make([]string, 0, len(peer.Groups))
	for _, group := range peer.Groups {
		if group.Name == allGroup {
			continue
		}
		names = append(names, group.Name)
	}

	return names
}

func (s *Service) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if apiKey == "" {
		return s.CreateHealthResponse(startTime, "error", "Service not configured: missing API token"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	if err := validateURL(url); err != nil {
		return s.CreateHealthResponse(startTime, "error", err.Error()), http.StatusBadRequest
	}

	if !strings.HasPrefix(apiKey, tokenPrefix) {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("invalid token format. Must start with '%s'", tokenPrefix)), http.StatusBadRequest
	}

	peers, responseTime, err := s.fetchPeers(healthCtx, url, apiKey)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", err.Error()), http.StatusServiceUnavailable
	}

	if len(peers) == 0 {
		return s.CreateHealthResponse(startTime, "warning", "Token sees no peers: use a token from an admin user"), http.StatusOK
	}

	info := s.fetchVersion(healthCtx, url, apiKey)

	onlineCount := 0
	for _, peer := range peers {
		if peer.Connected {
			onlineCount++
		}
	}

	extras := map[string]any{
		"responseTime":    responseTime.Milliseconds(),
		"updateAvailable": info.serverUpdateAvailable(),
		"details": map[string]any{
			"netbird": map[string]any{
				"total":           len(peers),
				"online":          onlineCount,
				"updateAvailable": info.serverUpdateAvailable(),
			},
		},
	}
	if info.ManagementCurrentVersion != "" {
		extras["version"] = info.ManagementCurrentVersion
	}

	message := fmt.Sprintf("%d/%d peers online", onlineCount, len(peers))
	if onlineCount == 0 {
		message = fmt.Sprintf("No peer connected (%d known)", len(peers))
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}

// serverUpdateAvailable reports the server badge the dashboard shows. A version
// the parser cannot read, for example "development", is no proof that the server
// is out of date, so such an answer produces no badge at all.
func (v versionInfo) serverUpdateAvailable() bool {
	if !v.ManagementUpdateAvailable {
		return false
	}
	if _, err := version.NewVersion(v.ManagementCurrentVersion); err != nil {
		return false
	}
	if _, err := version.NewVersion(v.ManagementAvailableVersion); err != nil {
		return false
	}

	return true
}

// GetPeers returns the peers of the network, ready for the dashboard payload, and
// whether the management server is older than the newest release. A failed
// version check gives no update badges and false, not an error.
func (s *Service) GetPeers(ctx context.Context, baseURL, apiKey string) ([]Peer, bool, error) {
	apiPeers, _, err := s.fetchPeers(ctx, baseURL, apiKey)
	if err != nil {
		return nil, false, err
	}

	info := s.fetchVersion(ctx, baseURL, apiKey)

	peers := make([]Peer, 0, len(apiPeers))
	for _, peer := range apiPeers {
		peers = append(peers, Peer{
			ID:              peer.ID,
			Name:            peer.Name,
			IP:              peer.IP,
			Online:          peer.Connected,
			LastSeen:        peer.LastSeen,
			OS:              peer.OS,
			Version:         peer.Version,
			UpdateAvailable: needsUpdate(peer.Version, info.ManagementAvailableVersion),
			Groups:          groupNames(peer),
			CityName:        peer.CityName,
			CountryCode:     peer.CountryCode,
			LoginExpired:    peer.LoginExpired,
		})
	}

	return peers, info.serverUpdateAvailable(), nil
}

// validateURL rejects an address that is not the absolute http(s) root of a
// management server, so a typo shows up as a status error instead of a vague
// request failure.
func validateURL(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return errors.New("URL has no host")
	}
	if strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/api") {
		return errors.New("URL must be the address of the management server without /api: dashbrr adds /api itself")
	}

	return nil
}
