// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package netbird

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// peerFixture is a recorded response of GET /api/peers with the real names, IPs,
// public addresses and cities replaced.
const peerFixture = `[
  {
    "id": "peer-host-1",
    "name": "host-1",
    "ip": "100.111.0.10",
    "connection_ip": "203.0.113.10",
    "connected": true,
    "last_seen": "2026-11-14T10:05:26.420578Z",
    "os": "Linux",
    "kernel_version": "6.1.0",
    "version": "0.60.0",
    "city_name": "Kiel",
    "country_code": "DE",
    "login_expired": false,
    "groups": [
      {"id": "group-all", "name": "All", "peers_count": 2},
      {"id": "group-homelab", "name": "homelab", "peers_count": 2}
    ]
  },
  {
    "id": "peer-host-2",
    "name": "host-2",
    "ip": "100.111.0.11",
    "connection_ip": "203.0.113.11",
    "connected": false,
    "last_seen": "2026-11-13T21:00:00Z",
    "os": "Windows",
    "version": "development",
    "city_name": "Berlin",
    "country_code": "DE",
    "login_expired": true,
    "groups": [
      {"id": "group-all", "name": "All", "peers_count": 2},
      {"id": "group-office", "name": "office", "peers_count": 1}
    ]
  }
]`

// versionFixture is a recorded response of GET /api/instance/version.
const versionFixture = `{
  "management_current_version": "0.60.0",
  "dashboard_available_version": "2.10.0",
  "management_available_version": "0.61.0",
  "management_update_available": true
}`

func newNetBirdTestService() *Service {
	return NewNetBirdService().(*Service)
}

// initTestCache gives the service a cache in a temporary directory, because the
// version answer is cached and each test must start cold.
func initTestCache(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if _, err := cache.InitCache(context.Background(), cache.Config{DataDir: t.TempDir()}); err != nil {
		t.Fatalf("InitCache: %v", err)
	}
}

type fixtureServer struct {
	*httptest.Server
	peerCalls    atomic.Int32
	versionCalls atomic.Int32
}

// newFixtureServer serves the recorded responses and records what the service
// sent: path, authorization header and host.
func newFixtureServer(t *testing.T, peers string, versionStatus int, versionBody string) *fixtureServer {
	t.Helper()

	fixture := &fixtureServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		fixture.peerCalls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Token nbp_testtoken" {
			http.Error(w, "unexpected authorization header: "+got, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(peers))
	})
	mux.HandleFunc("/api/instance/version", func(w http.ResponseWriter, _ *http.Request) {
		fixture.versionCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(versionStatus)
		_, _ = w.Write([]byte(versionBody))
	})

	fixture.Server = httptest.NewServer(mux)
	t.Cleanup(fixture.Close)
	return fixture
}

func TestCheckHealthOnlineCountComesFromConnected(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_testtoken")
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("status = %q, want online", health.Status)
	}
	if health.Message != "1/2 peers online" {
		t.Fatalf("message = %q, want 1/2 peers online", health.Message)
	}
	if !health.UpdateAvailable {
		t.Fatalf("updateAvailable = false, want true from the version call")
	}
	if health.Version != "0.60.0" {
		t.Fatalf("version = %q, want 0.60.0", health.Version)
	}

	details, ok := health.Details["netbird"].(map[string]any)
	if !ok {
		t.Fatalf("details.netbird missing: %#v", health.Details)
	}
	if details["online"] != 1 || details["total"] != 2 {
		t.Fatalf("details = %#v, want online 1 of total 2", details)
	}
}

func TestCheckHealthEmptyPeerListWarnsAboutTokenRights(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, `[]`, http.StatusOK, versionFixture)

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_testtoken")
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "warning" {
		t.Fatalf("status = %q, want warning", health.Status)
	}
	if health.Message != "Token sees no peers: use a token from an admin user" {
		t.Fatalf("message = %q", health.Message)
	}
}

func TestCheckHealthRejectedTokenIsAnError(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, `{"code":43,"message":"token invalid"}`, http.StatusOK, versionFixture)
	_ = server // the peer handler answers 401 when the header is not the expected one

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_wrongtoken")
	if statusCode != http.StatusServiceUnavailable {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusServiceUnavailable)
	}
	if health.Status != "error" {
		t.Fatalf("status = %q, want error", health.Status)
	}
}

func TestCheckHealthRejects401FromServer(t *testing.T) {
	initTestCache(t)
	mux := http.ServeMux{}
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
	})
	server := httptest.NewServer(&mux)
	defer server.Close()

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_testtoken")
	if statusCode != http.StatusServiceUnavailable || health.Status != "error" {
		t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
	}
}

func TestCheckHealthRejectsTokenWithoutPrefixBeforeAnyRequest(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "not-a-netbird-token")
	if statusCode != http.StatusBadRequest || health.Status != "error" {
		t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
	}
	if server.peerCalls.Load() != 0 || server.versionCalls.Load() != 0 {
		t.Fatalf("requests were sent for a rejected token: peers=%d version=%d",
			server.peerCalls.Load(), server.versionCalls.Load())
	}
}

func TestCheckHealthMissingTokenIsAnError(t *testing.T) {
	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), "http://127.0.0.1:1", "")
	if statusCode != http.StatusBadRequest || health.Status != "error" {
		t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
	}
}

func TestCheckHealthRequestsTheURLTheUserEntered(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)
	// A base URL with a sub path keeps the path, because a self-hosting server can
	// sit behind a reverse proxy.
	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL+"/", "nbp_testtoken")
	if statusCode != http.StatusOK || health.Status != "online" {
		t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
	}
	if server.peerCalls.Load() != 1 {
		t.Fatalf("peer calls = %d, want 1", server.peerCalls.Load())
	}
}

func TestGetPeersMapsFieldsAndHidesAllGroup(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)

	peers, err := newNetBirdTestService().GetPeers(t.Context(), server.URL, "nbp_testtoken")
	if err != nil {
		t.Fatalf("GetPeers: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("len(peers) = %d, want 2", len(peers))
	}

	first := peers[0]
	if first.Name != "host-1" || first.IP != "100.111.0.10" || !first.Online {
		t.Fatalf("first peer = %#v", first)
	}
	if first.CityName != "Kiel" || first.CountryCode != "DE" || first.OS != "Linux" {
		t.Fatalf("first peer location/os = %#v", first)
	}
	for _, group := range first.Groups {
		if group == allGroup {
			t.Fatalf("group %q must stay out of the payload: %#v", allGroup, first.Groups)
		}
	}
	if len(first.Groups) != 1 || first.Groups[0] != "homelab" {
		t.Fatalf("groups = %#v, want [homelab]", first.Groups)
	}
	if !first.UpdateAvailable {
		t.Fatalf("peer on 0.60.0 must get an update badge when 0.61.0 is available")
	}

	second := peers[1]
	if second.Online {
		t.Fatalf("second peer must be offline: %#v", second)
	}
	if second.UpdateAvailable {
		t.Fatalf("peer on %q must not get an update badge", second.Version)
	}
	if !second.LoginExpired {
		t.Fatalf("second peer must carry loginExpired: %#v", second)
	}
}

func TestGetPeersWithoutVersionEndpointShowsNoBadges(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusNotFound, `{"message":"not found"}`)

	peers, err := newNetBirdTestService().GetPeers(t.Context(), server.URL, "nbp_testtoken")
	if err != nil {
		t.Fatalf("GetPeers: %v", err)
	}
	for _, peer := range peers {
		if peer.UpdateAvailable {
			t.Fatalf("peer %q got an update badge although the version call failed", peer.Name)
		}
	}
}

func TestVersionAnswerIsCachedForOneHour(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)
	service := newNetBirdTestService()

	for i := 0; i < 3; i++ {
		if _, err := service.GetPeers(t.Context(), server.URL, "nbp_testtoken"); err != nil {
			t.Fatalf("GetPeers run %d: %v", i, err)
		}
	}

	// The first run fetches, and the cached JSON tells later runs apart from an
	// empty cache: three runs must leave exactly one version call.
	if got := server.versionCalls.Load(); got != 1 {
		t.Fatalf("version calls = %d, want 1 (cached for an hour)", got)
	}
	if got := server.peerCalls.Load(); got != 3 {
		t.Fatalf("peer calls = %d, want 3", got)
	}
}

func TestServerUpdateAvailable(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusOK, versionFixture)

	if !newNetBirdTestService().ServerUpdateAvailable(t.Context(), server.URL, "nbp_testtoken") {
		t.Fatalf("serverUpdateAvailable = false, want true")
	}
}

func TestServerUpdateAvailableIsFalseWhenVersionCallFails(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusInternalServerError, `{"message":"boom"}`)

	if newNetBirdTestService().ServerUpdateAvailable(t.Context(), server.URL, "nbp_testtoken") {
		t.Fatalf("serverUpdateAvailable = true, want false for a failed version call")
	}
}

func TestCheckHealthKeepsPeersWhenVersionCallFails(t *testing.T) {
	initTestCache(t)
	server := newFixtureServer(t, peerFixture, http.StatusNotFound, `{"message":"not found"}`)

	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_testtoken")
	if statusCode != http.StatusOK || health.Status != "online" {
		t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
	}
	if health.UpdateAvailable {
		t.Fatalf("updateAvailable = true, want false without a version answer")
	}
}

func TestNeedsUpdate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		peer      string
		available string
		want      bool
	}{
		{"older patch", "0.60.0", "0.61.0", true},
		{"same version", "0.61.0", "0.61.0", false},
		{"newer than release", "0.62.0", "0.61.0", false},
		{"development build", "development", "0.61.0", false},
		{"unparsable release", "0.60.0", "nightly", false},
		{"empty peer version", "", "0.61.0", false},
		{"no available version", "0.60.0", "", false},
		{"pre-release suffix", "0.61.0-rc1", "0.61.0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsUpdate(tc.peer, tc.available); got != tc.want {
				t.Fatalf("needsUpdate(%q, %q) = %v, want %v", tc.peer, tc.available, got, tc.want)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	for _, tc := range []struct {
		url     string
		wantErr bool
	}{
		{"https://api.netbird.io", false},
		{"https://netbird.example.com", false},
		{"http://192.0.2.10:33073", false},
		{"mydomain:33073", true},
		{"netbird.example.com", true},
		{"ftp://netbird.example.com", true},
	} {
		if err := ValidateURL(tc.url); (err != nil) != tc.wantErr {
			t.Fatalf("ValidateURL(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
		}
	}
}

func TestUnmarshalPeerFixture(t *testing.T) {
	// Guards the fixture against a typo: the recorded body must decode into the
	// API struct with the fields the payload needs.
	var peers []apiPeer
	if err := json.Unmarshal([]byte(peerFixture), &peers); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(peers) != 2 || peers[0].IP == "" || len(peers[0].Groups) != 2 {
		t.Fatalf("fixture decoded to unexpected shape: %#v", peers)
	}
}

// newVersionStubServer serves the peer fixture and hands every version request to
// the given handler, so a test can stall it, answer 404 or send a partial body.
func newVersionStubServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	versionCalls := &atomic.Int32{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(peerFixture))
	})
	mux.HandleFunc("/api/instance/version", func(w http.ResponseWriter, r *http.Request) {
		versionCalls.Add(1)
		handler(w, r)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, versionCalls
}

func TestCheckHealthSurvivesStallingVersionEndpoint(t *testing.T) {
	initTestCache(t)
	server, _ := newVersionStubServer(t, func(_ http.ResponseWriter, r *http.Request) {
		// Answer only after the caller gives up, the worst case for a shared deadline.
		<-r.Context().Done()
	})

	start := time.Now()
	health, statusCode := newNetBirdTestService().CheckHealth(t.Context(), server.URL, "nbp_testtoken")
	elapsed := time.Since(start)

	if statusCode != http.StatusOK || health.Status != "online" {
		t.Fatalf("status = %q (%d), want online (200): a slow version call must not fail the peer check", health.Status, statusCode)
	}
	if health.Message != "1/2 peers online" {
		t.Fatalf("message = %q, want 1/2 peers online", health.Message)
	}
	if health.UpdateAvailable {
		t.Fatalf("updateAvailable = true, want false without a version answer")
	}
	// The optional call gets versionTimeout, the whole check gets core.DefaultTimeout.
	if elapsed >= core.DefaultTimeout {
		t.Fatalf("health check took %s, must return before the %s deadline", elapsed, core.DefaultTimeout)
	}
}

func TestUnavailableVersionEndpointIsRemembered(t *testing.T) {
	initTestCache(t)
	server, versionCalls := newVersionStubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})

	service := newNetBirdTestService()
	if _, err := service.GetPeers(t.Context(), server.URL, "nbp_testtoken"); err != nil {
		t.Fatalf("GetPeers: %v", err)
	}
	if service.ServerUpdateAvailable(t.Context(), server.URL, "nbp_testtoken") {
		t.Fatalf("updateAvailable = true without a version endpoint")
	}
	if _, err := service.GetPeers(t.Context(), server.URL, "nbp_testtoken"); err != nil {
		t.Fatalf("second GetPeers: %v", err)
	}

	if got := versionCalls.Load(); got != 1 {
		t.Fatalf("version requests = %d, want 1: a 404 is cached for %s", got, versionMissTTL)
	}
}

func TestVersionAnswerWithoutNewestReleaseIsCached(t *testing.T) {
	initTestCache(t)
	server, versionCalls := newVersionStubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"management_current_version": "0.60.0"}`))
	})

	service := newNetBirdTestService()
	peers, err := service.GetPeers(t.Context(), server.URL, "nbp_testtoken")
	if err != nil {
		t.Fatalf("GetPeers: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("peers = %d, want 2", len(peers))
	}
	for _, peer := range peers {
		if peer.UpdateAvailable {
			t.Fatalf("peer %s shows an update badge without a release to compare against", peer.Name)
		}
	}
	if service.ServerUpdateAvailable(t.Context(), server.URL, "nbp_testtoken") {
		t.Fatalf("updateAvailable = true without a newer release")
	}

	if got := versionCalls.Load(); got != 1 {
		t.Fatalf("version requests = %d, want 1: an answer without the newest release is cached too", got)
	}
}
