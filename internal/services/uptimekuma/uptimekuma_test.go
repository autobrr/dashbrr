// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package uptimekuma

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildMetricsRequest_WithAPIKey(t *testing.T) {
	t.Parallel()

	endpoint, headers, err := buildMetricsRequest("http://localhost:3001", "uk_123_abc")
	if err != nil {
		t.Fatalf("buildMetricsRequest() error = %v", err)
	}

	if endpoint != "http://localhost:3001/metrics" {
		t.Fatalf("endpoint = %q, want %q", endpoint, "http://localhost:3001/metrics")
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("dashbrr:uk_123_abc"))
	if headers["Authorization"] != wantAuth {
		t.Fatalf("Authorization = %q, want %q", headers["Authorization"], wantAuth)
	}
}

func TestBuildMetricsRequest_WithURLCredentials(t *testing.T) {
	t.Parallel()

	endpoint, headers, err := buildMetricsRequest("http://user:pass@localhost:3001/kuma", "")
	if err != nil {
		t.Fatalf("buildMetricsRequest() error = %v", err)
	}

	if endpoint != "http://localhost:3001/kuma/metrics" {
		t.Fatalf("endpoint = %q, want %q", endpoint, "http://localhost:3001/kuma/metrics")
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if headers["Authorization"] != wantAuth {
		t.Fatalf("Authorization = %q, want %q", headers["Authorization"], wantAuth)
	}
}

func TestParseSummaryFromMetrics(t *testing.T) {
	t.Parallel()

	metrics := `# HELP monitor_status Monitor Status
monitor_status{monitor_id="1",monitor_name="API",monitor_type="http",monitor_url="https://api.example"} 1
monitor_status{monitor_id="2",monitor_name="DB",monitor_type="tcp",monitor_url="tcp://db.example:5432"} 0
monitor_response_time{monitor_id="1",monitor_name="API"} 42
monitor_response_time{monitor_id="2",monitor_name="DB"} -1`

	summary, err := parseSummaryFromMetrics(metrics)
	if err != nil {
		t.Fatalf("parseSummaryFromMetrics() error = %v", err)
	}

	if len(summary.Monitors) != 2 {
		t.Fatalf("monitor count = %d, want 2", len(summary.Monitors))
	}

	if summary.Monitors[0].Name != "API" {
		t.Fatalf("monitor[0].name = %q, want API", summary.Monitors[0].Name)
	}
	if summary.Monitors[0].Status != "up" {
		t.Fatalf("monitor[0].status = %q, want up", summary.Monitors[0].Status)
	}
	if summary.Monitors[0].ResponseTimeMs != 42 {
		t.Fatalf("monitor[0].responseTimeMs = %d, want 42", summary.Monitors[0].ResponseTimeMs)
	}

	if summary.Monitors[1].Name != "DB" {
		t.Fatalf("monitor[1].name = %q, want DB", summary.Monitors[1].Name)
	}
	if summary.Monitors[1].Status != "down" {
		t.Fatalf("monitor[1].status = %q, want down", summary.Monitors[1].Status)
	}
}

func TestParseSummaryFromMetrics_LabelValuesWithSpaces(t *testing.T) {
	t.Parallel()

	// Uptime Kuma passes monitor names through to labels unsanitized, so a
	// space in a name must not split the line. Label set and value formatting
	// match uptime-kuma server/prometheus.js.
	metrics := `monitor_status{monitor_id="1",monitor_name="My Media Server",monitor_type="http",monitor_url="https://media.example",monitor_hostname="null",monitor_port="null"} 1
monitor_response_time{monitor_id="1",monitor_name="My Media Server",monitor_type="http",monitor_url="https://media.example",monitor_hostname="null",monitor_port="null"} 12
monitor_status{monitor_id="2",monitor_name="Docker qbittorrent",monitor_type="docker",monitor_url="https://",monitor_hostname="null",monitor_port="null"} 0
monitor_response_time{monitor_id="2",monitor_name="Docker qbittorrent",monitor_type="docker",monitor_url="https://",monitor_hostname="null",monitor_port="null"} -1`

	summary, err := parseSummaryFromMetrics(metrics)
	if err != nil {
		t.Fatalf("parseSummaryFromMetrics() error = %v", err)
	}

	if len(summary.Monitors) != 2 {
		t.Fatalf("monitor count = %d, want 2", len(summary.Monitors))
	}
	if summary.Monitors[0].Name != "Docker qbittorrent" || summary.Monitors[0].Status != "down" {
		t.Fatalf("monitor[0] = %+v, want Docker qbittorrent down", summary.Monitors[0])
	}
	if summary.Monitors[1].Name != "My Media Server" || summary.Monitors[1].Status != "up" {
		t.Fatalf("monitor[1] = %+v, want My Media Server up", summary.Monitors[1])
	}
	if summary.Monitors[1].ResponseTimeMs != 12 {
		t.Fatalf("monitor[1].responseTimeMs = %d, want 12", summary.Monitors[1].ResponseTimeMs)
	}
}

func TestCheckHealth_DownMonitorReturnsWarning(t *testing.T) {
	t.Parallel()

	const apiKey = "uk_1_test"
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("dashbrr:"+apiKey))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			t.Fatalf("path = %q, want /metrics", r.URL.Path)
		}
		if r.Header.Get("Authorization") != wantAuth {
			t.Fatalf("Authorization = %q, want %q", r.Header.Get("Authorization"), wantAuth)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`monitor_status{monitor_id="1",monitor_name="API"} 1
monitor_status{monitor_id="2",monitor_name="DB"} 0`))
	}))
	defer server.Close()

	svc := NewUptimeKumaService().(*UptimeKumaService)
	health, statusCode := svc.CheckHealth(context.Background(), server.URL, apiKey)

	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
	if health.Status != "warning" {
		t.Fatalf("health.status = %q, want warning", health.Status)
	}
	if health.Message != "1 monitor(s) down" {
		t.Fatalf("health.message = %q, want %q", health.Message, "1 monitor(s) down")
	}
}
