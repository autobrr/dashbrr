// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/autobrr/dashbrr/internal/types"
)

func newQuiTestService() *QuiService {
	service := NewQuiService().(*QuiService)
	return service
}

func TestCheckHealth_InstancesAPIOnly(t *testing.T) {
	t.Parallel()

	for _, suffix := range []string{"", "/", "/qui", "/qui/"} {
		t.Run("url="+suffix, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != strings.TrimRight(suffix, "/")+"/api/instances" {
					_, _ = w.Write([]byte("<html>qui</html>"))
					return
				}
				if r.Header.Get("X-API-Key") != "test-key" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"id":1,"name":"Main","connected":true,"isActive":true}]`))
			}))
			defer server.Close()

			health, statusCode := newQuiTestService().CheckHealth(t.Context(), server.URL+suffix, "test-key")
			if statusCode != http.StatusOK || health.Status != "online" || health.Message != "1/1 active instances connected" {
				t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
			}
		})
	}
}

func TestCheckHealth_InstanceWarnings(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		body    string
		message string
	}{
		{"empty", `[]`, "Connected to qui, but no instances are configured"},
		{"inactive", `[{"isActive":false,"connected":true}]`, "Connected to qui, but no active instances are enabled"},
		{"disconnected", `[{"isActive":true,"connected":false}]`, "0/1 active instances connected"},
		{"credentials", `[{"isActive":true,"connected":true,"hasDecryptionError":true}]`, "1/1 active instances connected (1 with credential errors)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/qui/api/instances" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			health, statusCode := newQuiTestService().CheckHealth(t.Context(), server.URL+"/qui/", "test-key")
			if statusCode != http.StatusOK || health.Status != "warning" || health.Message != tc.message {
				t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
			}
		})
	}
}

func TestCheckHealth_APIFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		apiStatus  int
		body       string
		wantStatus string
		wantCode   int
		message    string
	}{
		{"unauthorized", http.StatusUnauthorized, "unauthorized", "error", http.StatusUnauthorized, "Invalid API key"},
		{"forbidden", http.StatusForbidden, "forbidden", "error", http.StatusForbidden, "Invalid API key"},
		{"missing route", http.StatusNotFound, "not found", "warning", http.StatusOK, "required API endpoint was not found"},
		{"server error", http.StatusInternalServerError, "internal error", "warning", http.StatusOK, "failed to query instances"},
		{"html", http.StatusOK, "<html>qui</html>", "warning", http.StatusOK, "failed to parse response"},
		{"wrong shape", http.StatusOK, `{"status":"ok"}`, "warning", http.StatusOK, "failed to parse response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/qui/api/instances" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.apiStatus)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			health, statusCode := newQuiTestService().CheckHealth(t.Context(), server.URL+"/qui", "test-key")
			if statusCode != tc.wantCode || health.Status != tc.wantStatus || !strings.Contains(health.Message, tc.message) {
				t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
			}
		})
	}
}

func TestCheckHealth_ConfigurationAndConnectionErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()

	for _, tc := range []struct {
		name       string
		url        string
		apiKey     string
		wantStatus string
		wantCode   int
		message    string
	}{
		{"missing URL", " ", "test-key", "error", http.StatusBadRequest, "Service not configured: missing URL"},
		{"missing key", server.URL, " ", "error", http.StatusBadRequest, "Service not configured: missing API key"},
		{"unreachable", server.URL, "test-key", "offline", http.StatusOK, "Failed to connect to qui:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			health, statusCode := newQuiTestService().CheckHealth(t.Context(), tc.url, tc.apiKey)
			if statusCode != tc.wantCode || health.Status != tc.wantStatus || !strings.HasPrefix(health.Message, tc.message) {
				t.Fatalf("statusCode = %d, health = %#v", statusCode, health)
			}
		})
	}
}

func TestCheckHealth_InvalidAPIKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/instances":
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newQuiTestService()
	health, statusCode := service.CheckHealth(context.Background(), server.URL, "bad-key")

	if statusCode != http.StatusUnauthorized {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusUnauthorized)
	}
	if health.Status != "error" {
		t.Fatalf("health.Status = %q, want %q", health.Status, "error")
	}
}

func TestGetAggregatedTransferInfo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/instances/1/transfer-info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"connection_status":"connected",
				"dht_nodes":72,
				"dl_info_data":1000,
				"dl_info_speed":1200,
				"dl_rate_limit":0,
				"up_info_data":500,
				"up_info_speed":300,
				"up_rate_limit":0
			}`))
		case "/api/instances/1/torrents":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"serverState": {
					"alltime_dl": 5000,
					"alltime_ul": 3000
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newQuiTestService()

	instances := []types.QuiInstance{
		{ID: 1, Name: "Main", IsActive: true, Connected: true},
		{ID: 2, Name: "Backup", IsActive: true, Connected: false},
		{ID: 3, Name: "Disabled", IsActive: false, Connected: true},
	}

	summary, transfers := service.GetAggregatedTransferInfo(
		context.Background(),
		server.URL,
		"test-key",
		instances,
	)

	if summary.TotalInstances != 3 {
		t.Fatalf("summary.TotalInstances = %d, want 3", summary.TotalInstances)
	}
	if summary.ActiveInstances != 2 {
		t.Fatalf("summary.ActiveInstances = %d, want 2", summary.ActiveInstances)
	}
	if summary.ConnectedInstances != 1 {
		t.Fatalf("summary.ConnectedInstances = %d, want 1", summary.ConnectedInstances)
	}
	if summary.DownloadSpeed != 1200 || summary.UploadSpeed != 300 {
		t.Fatalf("unexpected speed totals: dl=%d up=%d", summary.DownloadSpeed, summary.UploadSpeed)
	}
	if summary.Downloaded != 5000 || summary.Uploaded != 3000 {
		t.Fatalf("unexpected data totals: dl=%d up=%d", summary.Downloaded, summary.Uploaded)
	}
	if len(transfers) != 2 {
		t.Fatalf("len(transfers) = %d, want 2", len(transfers))
	}
	if transfers[0].InstanceID != 1 || transfers[1].InstanceID != 2 {
		t.Fatalf("unexpected transfer ordering: %#v", transfers)
	}
	if transfers[0].Downloaded != 5000 || transfers[0].Uploaded != 3000 {
		t.Fatalf("unexpected transfer data totals: dl=%d up=%d", transfers[0].Downloaded, transfers[0].Uploaded)
	}
}

func TestGetAggregatedTransferInfo_FallsBackToSessionData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/instances/1/transfer-info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"connection_status":"connected",
				"dht_nodes":12,
				"dl_info_data":222,
				"dl_info_speed":40,
				"dl_rate_limit":0,
				"up_info_data":333,
				"up_info_speed":50,
				"up_rate_limit":0
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newQuiTestService()

	instances := []types.QuiInstance{
		{ID: 1, Name: "Main", IsActive: true, Connected: true},
	}

	summary, transfers := service.GetAggregatedTransferInfo(
		context.Background(),
		server.URL,
		"test-key",
		instances,
	)

	if summary.Downloaded != 222 || summary.Uploaded != 333 {
		t.Fatalf("fallback totals mismatch: dl=%d up=%d", summary.Downloaded, summary.Uploaded)
	}
	if len(transfers) != 1 {
		t.Fatalf("len(transfers) = %d, want 1", len(transfers))
	}
	if transfers[0].Downloaded != 222 || transfers[0].Uploaded != 333 {
		t.Fatalf("fallback transfer totals mismatch: dl=%d up=%d", transfers[0].Downloaded, transfers[0].Uploaded)
	}
}

func TestGetAggregatedTransferInfo_UsesCachedAllTimeTotalsOnTransientFailure(t *testing.T) {
	t.Parallel()

	var torrentsRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/instances/1/transfer-info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"connection_status":"connected",
				"dht_nodes":9,
				"dl_info_data":100,
				"dl_info_speed":20,
				"dl_rate_limit":0,
				"up_info_data":200,
				"up_info_speed":30,
				"up_rate_limit":0
			}`))
		case "/api/instances/1/torrents":
			current := torrentsRequests.Add(1)
			if current == 1 {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"serverState": {
						"alltime_dl": 9999,
						"alltime_ul": 8888
					}
				}`))
				return
			}
			http.Error(w, "upstream timeout", http.StatusGatewayTimeout)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newQuiTestService()
	instances := []types.QuiInstance{
		{ID: 1, Name: "Main", IsActive: true, Connected: true},
	}

	firstSummary, _ := service.GetAggregatedTransferInfo(
		context.Background(),
		server.URL,
		"test-key",
		instances,
	)
	if firstSummary.Downloaded != 9999 || firstSummary.Uploaded != 8888 {
		t.Fatalf("first totals mismatch: dl=%d up=%d", firstSummary.Downloaded, firstSummary.Uploaded)
	}

	secondSummary, secondTransfers := service.GetAggregatedTransferInfo(
		context.Background(),
		server.URL,
		"test-key",
		instances,
	)
	if secondSummary.Downloaded != 9999 || secondSummary.Uploaded != 8888 {
		t.Fatalf("cached totals mismatch: dl=%d up=%d", secondSummary.Downloaded, secondSummary.Uploaded)
	}
	if len(secondTransfers) != 1 {
		t.Fatalf("len(secondTransfers) = %d, want 1", len(secondTransfers))
	}
	if secondTransfers[0].Downloaded != 9999 || secondTransfers[0].Uploaded != 8888 {
		t.Fatalf("cached transfer totals mismatch: dl=%d up=%d", secondTransfers[0].Downloaded, secondTransfers[0].Uploaded)
	}
	if secondSummary.DownloadSpeed != 20 || secondSummary.UploadSpeed != 30 {
		t.Fatalf("speed totals mismatch: dl=%d up=%d", secondSummary.DownloadSpeed, secondSummary.UploadSpeed)
	}
}

func TestCheckHealth_SummarizesInstanceState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/instances":
			if r.Header.Get("X-API-Key") != "test-key" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":1,"name":"Main","connected":true,"isActive":true},
				{"id":2,"name":"Backup","connected":false,"isActive":true}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newQuiTestService()
	health, statusCode := service.CheckHealth(context.Background(), server.URL, "test-key")

	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "warning" {
		t.Fatalf("health.Status = %q, want %q", health.Status, "warning")
	}
	if health.Message == "" {
		t.Fatal("health.Message should not be empty")
	}
	if health.Details == nil || health.Details["qui"] == nil {
		t.Fatal("health.Details.qui should be set")
	}

	quiDetails, ok := health.Details["qui"].(map[string]any)
	if !ok {
		t.Fatalf("health.Details[\"qui\"] type = %T, want map[string]interface{}", health.Details["qui"])
	}
	if _, hasSummary := quiDetails["summary"]; hasSummary {
		t.Fatal("health details should not include qui.summary to avoid overwriting overview metrics")
	}
}

// Current qui serves the all-time totals from transfer-info itself. Requesting the
// torrent list on top of that makes qui filter and sort its whole torrent set for
// two numbers it already sent, so the extra request must not happen at all.
func TestGetAggregatedTransferInfo_UsesTransferInfoTotals(t *testing.T) {
	t.Parallel()

	var torrentRequests atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/instances/1/transfer-info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"connection_status":"connected",
				"dht_nodes":72,
				"dl_info_data":1000,
				"dl_info_speed":1200,
				"dl_rate_limit":0,
				"up_info_data":500,
				"up_info_speed":300,
				"up_rate_limit":0,
				"alltime_dl":5000,
				"alltime_ul":3000
			}`))
		case "/api/instances/1/torrents":
			torrentRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"serverState":{"alltime_dl":1,"alltime_ul":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	summary, transfers := newQuiTestService().GetAggregatedTransferInfo(
		context.Background(),
		server.URL,
		"test-key",
		[]types.QuiInstance{{ID: 1, Name: "Main", IsActive: true, Connected: true}},
	)

	if got := torrentRequests.Load(); got != 0 {
		t.Fatalf("torrent list requests = %d, want 0", got)
	}
	if summary.Downloaded != 5000 || summary.Uploaded != 3000 {
		t.Fatalf("unexpected data totals: dl=%d up=%d", summary.Downloaded, summary.Uploaded)
	}
	if summary.DownloadSpeed != 1200 || summary.UploadSpeed != 300 {
		t.Fatalf("unexpected speed totals: dl=%d up=%d", summary.DownloadSpeed, summary.UploadSpeed)
	}
	if len(transfers) != 1 || transfers[0].Downloaded != 5000 || transfers[0].Uploaded != 3000 {
		t.Fatalf("unexpected transfers: %#v", transfers)
	}
}
