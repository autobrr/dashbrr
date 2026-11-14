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

// versionTTL caches the answer of /api/instance/version for one hour.
const versionTTL = time.Hour

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

func init() {
	models.NewNetBirdService = NewNetBirdService
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

func peerURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/api/peers"
}

func versionURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/api/instance/version"
}

func (s *Service) fetchPeers(ctx context.Context, baseURL, apiKey string) ([]apiPeer, time.Duration, error) {
	startTime := time.Now()

	if !strings.HasPrefix(apiKey, tokenPrefix) {
		return nil, 0, fmt.Errorf("invalid token format. Must start with '%s'", tokenPrefix)
	}

	headers := map[string]string{
		"Authorization": "Token " + apiKey,
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, peerURL(baseURL), headers, nil)
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
// numbers, so one answer covers the peers and the server.
//
// The answer is cached in the version slot of the service cache for one hour, so
// the 60s peer poll does not call the endpoint on every run.
func (s *Service) fetchVersion(ctx context.Context, baseURL, apiKey string) (*versionInfo, error) {
	if cached := s.GetVersionFromCache(ctx, baseURL); cached != "" {
		var info versionInfo
		if err := json.Unmarshal([]byte(cached), &info); err != nil {
			info = versionInfo{ManagementCurrentVersion: cached, ManagementAvailableVersion: cached}
		}
		return &info, nil
	}

	headers := map[string]string{
		"Authorization": "Token " + apiKey,
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, versionURL(baseURL), headers, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed (Status %d): %s", resp.StatusCode, string(body))
	}

	var info versionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if info.ManagementAvailableVersion != "" {
		if encoded, err := json.Marshal(info); err == nil {
			// A cache failure must not fail the health check; the next poll tries again.
			_ = s.CacheVersion(ctx, baseURL, string(encoded), versionTTL)
		}
	}

	return &info, nil
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

	if err := ValidateURL(url); err != nil {
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

	info, versionErr := s.fetchVersion(healthCtx, url, apiKey)
	if versionErr != nil {
		// A missing version endpoint must not break the peer list (NetBird Cloud
		// does not document it), so the status stays online and no badge shows.
		info = nil
	}

	onlineCount := 0
	for _, peer := range peers {
		if peer.Connected {
			onlineCount++
		}
	}

	extras := map[string]any{
		"responseTime":    responseTime.Milliseconds(),
		"updateAvailable": info != nil && info.ManagementUpdateAvailable,
	}

	if info != nil {
		extras["version"] = info.ManagementCurrentVersion
	}

	details := map[string]any{
		"netbird": map[string]any{
			"total":           len(peers),
			"online":          onlineCount,
			"updateAvailable": info != nil && info.ManagementUpdateAvailable,
			"serverVersion":   "",
		},
	}
	if info != nil {
		serverDetails := details["netbird"].(map[string]any)
		serverDetails["serverVersion"] = info.ManagementCurrentVersion
	}
	extras["details"] = details

	message := fmt.Sprintf("%d/%d peers online", onlineCount, len(peers))
	if onlineCount == 0 {
		message = fmt.Sprintf("No peer connected (%d known)", len(peers))
	}

	return s.CreateHealthResponse(startTime, "online", message, extras), http.StatusOK
}

// GetPeers returns the peers of the network, ready for the dashboard payload.
func (s *Service) GetPeers(ctx context.Context, baseURL, apiKey string) ([]Peer, error) {
	apiPeers, _, err := s.fetchPeers(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}

	info, err := s.fetchVersion(ctx, baseURL, apiKey)
	if err != nil {
		info = nil
	}

	availableVersion := ""
	if info != nil {
		availableVersion = info.ManagementAvailableVersion
	}

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
			UpdateAvailable: needsUpdate(peer.Version, availableVersion),
			Groups:          groupNames(peer),
			CityName:        peer.CityName,
			CountryCode:     peer.CountryCode,
			LoginExpired:    peer.LoginExpired,
		})
	}

	return peers, nil
}

// ServerUpdateAvailable reports whether the management server is older than the
// newest release. A failed version call answers false, so the dashboard shows no
// update hint instead of an error.
func (s *Service) ServerUpdateAvailable(ctx context.Context, baseURL, apiKey string) bool {
	info, err := s.fetchVersion(ctx, baseURL, apiKey)
	if err != nil || info == nil {
		return false
	}

	return info.ManagementUpdateAvailable
}

// ValidateURL rejects an address that is not an absolute http(s) URL, so a typo
// in the management URL shows up as a status error instead of a vague request failure.
func ValidateURL(baseURL string) error {
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

	return nil
}
