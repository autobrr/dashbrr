// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package nzbget

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGetStatus_UsesJSONRPCAndAuthHeader(t *testing.T) {
	t.Parallel()

	var (
		mu            sync.Mutex
		calledMethod  string
		authHeader    string
		requestMethod string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		authHeader = r.Header.Get("Authorization")
		requestMethod = r.Method
		if r.URL.Path != "/jsonrpc" {
			t.Fatalf("path = %s, want /jsonrpc", r.URL.Path)
		}
		if requestMethod != http.MethodPost {
			t.Fatalf("request method = %s, want POST", requestMethod)
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		calledMethod = req.Method

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": {
				"DownloadPaused": false,
				"PostPaused": false,
				"ScanPaused": false,
				"QuotaReached": false,
				"RemainingSizeLo": 1,
				"RemainingSizeHi": 0,
				"FreeDiskSpaceLo": 2,
				"FreeDiskSpaceHi": 0,
				"DownloadRateLo": 3,
				"DownloadRateHi": 0
			},
			"error": null
		}`))
	}))
	defer server.Close()

	service := NewNzbgetService().(*NzbgetService)
	status, err := service.GetStatus(context.Background(), server.URL, "admin:secret")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}

	if calledMethod != "status" {
		t.Fatalf("rpc method = %s, want status", calledMethod)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	if authHeader != wantAuth {
		t.Fatalf("authorization header = %q, want %q", authHeader, wantAuth)
	}
	if status.DownloadRateLo != 3 {
		t.Fatalf("DownloadRateLo = %d, want 3", status.DownloadRateLo)
	}
}

func TestGetSummary_CombinesStatusQueueAndFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "status":
			_, _ = w.Write([]byte(`{
				"result": {
					"DownloadPaused": true,
					"PostPaused": false,
					"ScanPaused": false,
					"QuotaReached": false,
					"RemainingSizeLo": 1000,
					"RemainingSizeHi": 0,
					"FreeDiskSpaceLo": 2000,
					"FreeDiskSpaceHi": 0,
					"DownloadRateLo": 1024,
					"DownloadRateHi": 0
				},
				"error": null
			}`))
		case "listgroups":
			_, _ = w.Write([]byte(`{
				"result": [
					{"NZBID": 1, "NZBName": "Movie.A", "Status": "DOWNLOADING", "RemainingSizeMB": 500},
					{"NZBID": 2, "NZBName": "Movie.B", "Status": "QUEUED", "RemainingSizeMB": 200}
				],
				"error": null
			}`))
		case "history":
			_, _ = w.Write([]byte(`{
				"result": [
					{"NZBID": 11, "Name": "Ok", "Status": "SUCCESS/ALL", "HistoryTime": 1},
					{"NZBID": 12, "Name": "Warn", "Status": "WARNING/SCRIPT", "HistoryTime": 4},
					{"NZBID": 13, "Name": "Fail", "Status": "FAILURE/UNPACK", "HistoryTime": 3}
				],
				"error": null
			}`))
		default:
			t.Fatalf("unexpected method: %s", req.Method)
		}
	}))
	defer server.Close()

	service := NewNzbgetService().(*NzbgetService)
	summary, err := service.GetSummary(context.Background(), server.URL, "admin:secret")
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}

	if !summary.Status.DownloadPaused {
		t.Fatal("expected paused summary status")
	}
	if len(summary.Queue) != 2 {
		t.Fatalf("queue len = %d, want 2", len(summary.Queue))
	}
	if summary.FailedCount != 2 {
		t.Fatalf("failedCount = %d, want 2", summary.FailedCount)
	}
	if len(summary.RecentFailures) != 2 {
		t.Fatalf("recentFailures len = %d, want 2", len(summary.RecentFailures))
	}
	if summary.RecentFailures[0].NZBID != 12 {
		t.Fatalf("recentFailures[0].NZBID = %d, want 12", summary.RecentFailures[0].NZBID)
	}
}

func TestParseCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawURL   string
		apiKey   string
		wantUser string
		wantPass string
		wantBase string
		wantErr  string
	}{
		{
			name:     "url credentials win",
			rawURL:   "http://john:doe@localhost:6789",
			apiKey:   "ignored",
			wantUser: "john",
			wantPass: "doe",
			wantBase: "http://localhost:6789",
		},
		{
			name:     "api key as user pass",
			rawURL:   "http://localhost:6789",
			apiKey:   "alice:secret",
			wantUser: "alice",
			wantPass: "secret",
			wantBase: "http://localhost:6789",
		},
		{
			name:     "api key as control password",
			rawURL:   "http://localhost:6789",
			apiKey:   "control-password",
			wantUser: "nzbget",
			wantPass: "control-password",
			wantBase: "http://localhost:6789",
		},
		{
			name:    "missing creds",
			rawURL:  "http://localhost:6789",
			apiKey:  "",
			wantErr: "control password is required",
		},
		{
			name:    "invalid url",
			rawURL:  "localhost:6789",
			apiKey:  "x",
			wantErr: "invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, user, pass, err := parseCredentials(tt.rawURL, tt.apiKey)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if base != tt.wantBase {
				t.Fatalf("base = %q, want %q", base, tt.wantBase)
			}
			if user != tt.wantUser {
				t.Fatalf("user = %q, want %q", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Fatalf("pass = %q, want %q", pass, tt.wantPass)
			}
		})
	}
}

