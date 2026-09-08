// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/autobrr/dashbrr/internal/models"
)

// Top-level scalar fields from arbitrary JSON payloads are exposed via
// details.general for display (#87); nested values and the health-check
// status/message keys are not.
func TestCheckHealth_ExposesScalarFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"username": "soup",
			"ratio": "2.34",
			"seeding": 120,
			"can_download": true,
			"status": "ok",
			"message": "hello",
			"nested": {"skip": "me"},
			"list": [1, 2]
		}`))
	}))
	defer server.Close()

	service := NewGeneralService().(*GeneralService)
	health, code := service.CheckHealth(context.Background(), server.URL, "")

	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q", health.Status, "online")
	}

	fields, ok := health.Details["general"].(map[string]any)
	if !ok {
		t.Fatalf("Details[\"general\"] missing, Details = %#v", health.Details)
	}
	for _, key := range []string{"username", "ratio", "seeding", "can_download"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("fields[%q] missing", key)
		}
	}
	for _, key := range []string{"status", "message", "nested", "list"} {
		if _, ok := fields[key]; ok {
			t.Errorf("fields[%q] should be excluded", key)
		}
	}
}

// Backward compatibility: a nil CustomServiceConfig must behave exactly like
// the pre-D2 general service - apiKey as a Bearer header, status/message
// parsed from the JSON body, other scalar fields under details.general.
func TestCheckHealth_BackwardCompatNilConfig(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"1.2.3"}`))
	}))
	defer server.Close()

	service := NewGeneralService().(*GeneralService)
	health, code := service.CheckHealth(context.Background(), server.URL, "my-api-key")

	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if gotAuth != "Bearer my-api-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer my-api-key")
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q", health.Status, "online")
	}

	fields, ok := health.Details["general"].(map[string]any)
	if !ok {
		t.Fatalf("Details[\"general\"] missing, Details = %#v", health.Details)
	}
	if fields["version"] != "1.2.3" {
		t.Fatalf("fields[version] = %v, want %q", fields["version"], "1.2.3")
	}
}

// Every auth mode must be observed on the wire exactly as configured.
func TestCheckHealth_AuthModes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		apiKey string
		auth   *models.CustomAuthConfig
		check  func(t *testing.T, r *http.Request)
	}{
		{
			name:   "none",
			apiKey: "secret",
			auth:   &models.CustomAuthConfig{Mode: "none"},
			check: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "" {
					t.Errorf("Authorization = %q, want empty", got)
				}
			},
		},
		{
			name:   "header",
			apiKey: "secret",
			auth:   &models.CustomAuthConfig{Mode: "header", HeaderName: "X-Api-Token"},
			check: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("X-Api-Token"); got != "secret" {
					t.Errorf("X-Api-Token = %q, want %q", got, "secret")
				}
			},
		},
		{
			name:   "query",
			apiKey: "secret",
			auth:   &models.CustomAuthConfig{Mode: "query", QueryParam: "token"},
			check: func(t *testing.T, r *http.Request) {
				if got := r.URL.Query().Get("token"); got != "secret" {
					t.Errorf("query token = %q, want %q", got, "secret")
				}
			},
		},
		{
			name: "basic",
			auth: &models.CustomAuthConfig{Mode: "basic", Username: "user", Password: "pass"},
			check: func(t *testing.T, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != "user" || pass != "pass" {
					t.Errorf("BasicAuth = (%q, %q, %v), want (user, pass, true)", user, pass, ok)
				}
			},
		},
		{
			name: "bearer",
			auth: &models.CustomAuthConfig{Mode: "bearer", Token: "tok123"},
			check: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
					t.Errorf("Authorization = %q, want %q", got, "Bearer tok123")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotReq *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotReq = r.Clone(r.Context())
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"up"}`))
			}))
			defer server.Close()

			cfg := &models.CustomServiceConfig{
				Auth: tc.auth,
				Health: &models.CustomHealthConfig{
					Path:       "/health",
					StatusPath: "status",
					OKValues:   []string{"up"},
				},
			}

			service := NewGeneralService().(*GeneralService)
			health, code := service.Engine.CheckHealth(context.Background(), server.URL, tc.apiKey, cfg)
			if code != http.StatusOK {
				t.Fatalf("code = %d, want %d", code, http.StatusOK)
			}
			if health.Status != "online" {
				t.Fatalf("Status = %q, want online", health.Status)
			}
			if gotReq == nil {
				t.Fatal("handler was not invoked")
			}
			tc.check(t, gotReq)
		})
	}
}

// Status mapping: ok/warn values map case-insensitively, booleans/numbers are
// stringified, anything else falls to offline.
func TestCheckHealth_StatusMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"ok", `{"state":"Good"}`, "online"},
		{"warn", `{"state":"DEGRADED"}`, "warning"},
		{"offline", `{"state":"dead"}`, "offline"},
		{"bool_true_is_ok", `{"state":true}`, "online"},
		{"missing_path", `{"other":"x"}`, "offline"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.payload))
			}))
			defer server.Close()

			cfg := &models.CustomServiceConfig{
				Health: &models.CustomHealthConfig{
					Path:       "/",
					StatusPath: "state",
					OKValues:   []string{"good", "true"},
					WarnValues: []string{"degraded"},
				},
			}

			service := NewGeneralService().(*GeneralService)
			health, _ := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
			if health.Status != tc.want {
				t.Fatalf("Status = %q, want %q", health.Status, tc.want)
			}
		})
	}
}

// StatusPath empty falls back to plain HTTP 2xx = online.
func TestCheckHealth_NoStatusPathUsesHTTPStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Health: &models.CustomHealthConfig{Path: "/"},
	}

	service := NewGeneralService().(*GeneralService)
	health, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want %d", code, http.StatusInternalServerError)
	}
	if health.Status != "offline" {
		t.Fatalf("Status = %q, want offline", health.Status)
	}
}

func TestCheckHealth_VersionPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"info":{"version":"9.9.9"}}`))
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Health: &models.CustomHealthConfig{Path: "/", VersionPath: "info.version"},
	}

	service := NewGeneralService().(*GeneralService)
	health, _ := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if health.Version != "9.9.9" {
		t.Fatalf("Version = %q, want %q", health.Version, "9.9.9")
	}
}

// Login step: a Set-Cookie is captured and injected as a Cookie header on the
// following request; the login endpoint is hit exactly once.
func TestCheckHealth_LoginCookieCapture(t *testing.T) {
	t.Parallel()

	var loginHits int32
	var healthCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			atomic.AddInt32(&loginHits, 1)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123"})
			w.WriteHeader(http.StatusOK)
		case "/health":
			if c, err := r.Cookie("session"); err == nil {
				healthCookie = c.Value
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Login: &models.CustomLoginConfig{
			Method:        "POST",
			Path:          "/login",
			CaptureCookie: "session",
			InjectAs:      "cookie",
			InjectName:    "session",
		},
		Health: &models.CustomHealthConfig{Path: "/health"},
	}

	service := NewGeneralService().(*GeneralService)
	health, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want online", health.Status)
	}
	if got := atomic.LoadInt32(&loginHits); got != 1 {
		t.Fatalf("loginHits = %d, want 1", got)
	}
	if healthCookie != "abc123" {
		t.Fatalf("healthCookie = %q, want %q", healthCookie, "abc123")
	}
}

// Login step: a JSON token is captured and injected as a Bearer header; the
// login request itself uses the configured content type and body.
func TestCheckHealth_LoginJSONTokenCapture(t *testing.T) {
	t.Parallel()

	var loginHits int32
	var healthAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			atomic.AddInt32(&loginHits, 1)
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("login Content-Type = %q, want application/json", got)
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"user":"a"}` {
				t.Errorf("login body = %q, want %q", string(body), `{"user":"a"}`)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"tok-xyz"}`))
		case "/health":
			healthAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Login: &models.CustomLoginConfig{
			Method:          "POST",
			Path:            "/login",
			ContentType:     "application/json",
			Body:            `{"user":"a"}`,
			CaptureJSONPath: "token",
			InjectAs:        "bearer",
		},
		Health: &models.CustomHealthConfig{Path: "/health"},
	}

	service := NewGeneralService().(*GeneralService)
	_, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if got := atomic.LoadInt32(&loginHits); got != 1 {
		t.Fatalf("loginHits = %d, want 1", got)
	}
	if healthAuth != "Bearer tok-xyz" {
		t.Fatalf("healthAuth = %q, want %q", healthAuth, "Bearer tok-xyz")
	}
}

// A login POST must be sent with its configured (non-JSON) content type and
// exact body.
func TestLogin_CustomContentType(t *testing.T) {
	t.Parallel()

	var gotContentType, gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			gotContentType = r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "xyz"})
			w.WriteHeader(http.StatusOK)
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Login: &models.CustomLoginConfig{
			Method:        "POST",
			Path:          "/login",
			ContentType:   "application/x-www-form-urlencoded",
			Body:          "user=a&pass=b",
			CaptureCookie: "sid",
			InjectAs:      "cookie",
			InjectName:    "sid",
		},
		Health: &models.CustomHealthConfig{Path: "/health"},
	}

	service := NewGeneralService().(*GeneralService)
	_, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want %q", gotContentType, "application/x-www-form-urlencoded")
	}
	if gotBody != "user=a&pass=b" {
		t.Fatalf("body = %q, want %q", gotBody, "user=a&pass=b")
	}
}

// A 401 on a request past login must invalidate the cached login value and
// retry exactly once (not loop).
func TestCheckHealth_ReloginOnceOn401(t *testing.T) {
	t.Parallel()

	var loginHits int32
	var healthHits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			n := atomic.AddInt32(&loginHits, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"token":"tok-%d"}`, n)
		case "/health":
			n := atomic.AddInt32(&healthHits, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Login: &models.CustomLoginConfig{
			Path:            "/login",
			CaptureJSONPath: "token",
			InjectAs:        "bearer",
		},
		Health: &models.CustomHealthConfig{Path: "/health"},
	}

	service := NewGeneralService().(*GeneralService)
	health, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want online", health.Status)
	}
	if got := atomic.LoadInt32(&loginHits); got != 2 {
		t.Fatalf("loginHits = %d, want 2 (initial + exactly one relogin)", got)
	}
	if got := atomic.LoadInt32(&healthHits); got != 2 {
		t.Fatalf("healthHits = %d, want 2", got)
	}
}

// FetchStats extracts and formats every configured stat by JSON path.
func TestFetchStats_Formats(t *testing.T) {
	t.Parallel()

	const payload = `{
		"diskFree": 1610612736,
		"uptimeSeconds": 93784,
		"cpuPercent": 42.567,
		"totalDownloads": 1234567,
		"label": "hello world",
		"zeroBytes": 0,
		"zeroDuration": 0,
		"count": 42
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Health: &models.CustomHealthConfig{Path: "/"},
		Stats: []models.CustomStatConfig{
			{Label: "Disk Free", Path: "diskFree", Format: "bytes"},
			{Label: "Uptime", Path: "uptimeSeconds", Format: "duration"},
			{Label: "CPU", Path: "cpuPercent", Format: "percent"},
			{Label: "Downloads", Path: "totalDownloads", Format: "number"},
			{Label: "Label", Path: "label", Format: "text"},
			{Label: "Zero Bytes", Path: "zeroBytes", Format: "bytes"},
			{Label: "Zero Duration", Path: "zeroDuration", Format: "duration"},
			{Label: "Count", Path: "count", Format: ""},
		},
	}

	service := NewGeneralService().(*GeneralService)
	stats, err := service.FetchStats(context.Background(), server.URL, "", cfg)
	if err != nil {
		t.Fatalf("FetchStats returned error: %v", err)
	}
	if len(stats) != 8 {
		t.Fatalf("len(stats) = %d, want 8", len(stats))
	}

	want := map[string]string{
		"Disk Free":     "1.5 GiB",
		"Uptime":        "1d 2h 3m",
		"CPU":           "42.6%",
		"Downloads":     "1,234,567",
		"Label":         "hello world",
		"Zero Bytes":    "0.0 B",
		"Zero Duration": "0s",
		"Count":         "42",
	}
	for label, wantDisplay := range want {
		got, ok := stats[label]
		if !ok {
			t.Errorf("missing stat %q", label)
			continue
		}
		if got.Display != wantDisplay {
			t.Errorf("stats[%q].Display = %q, want %q", label, got.Display, wantDisplay)
		}
	}
}

// RunAction hits the configured method+path+body; an unknown id is an error.
func TestRunAction(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Actions: []models.CustomActionConfig{
			{ID: "restart", Label: "Restart", Method: "POST", Path: "/restart", Body: `{"force":true}`},
		},
	}

	service := NewGeneralService().(*GeneralService)
	result, err := service.RunAction(context.Background(), server.URL, "", cfg, "restart")
	if err != nil {
		t.Fatalf("RunAction returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/restart" {
		t.Errorf("path = %q, want /restart", gotPath)
	}
	if gotBody != `{"force":true}` {
		t.Errorf("body = %q, want %q", gotBody, `{"force":true}`)
	}
	if result.Status != http.StatusAccepted {
		t.Errorf("Status = %d, want %d", result.Status, http.StatusAccepted)
	}
	if result.Body != `{"ok":true}` {
		t.Errorf("Body = %q, want %q", result.Body, `{"ok":true}`)
	}

	if _, err := service.RunAction(context.Background(), server.URL, "", cfg, "unknown-id"); err == nil {
		t.Fatal("expected error for unknown action id")
	}
}

// The login body's {{username}}/{{password}} placeholders must be substituted
// from cfg.Auth even when Auth.Mode is "none" - the qBittorrent preset
// (username={{username}}&password={{password}}) relies on Auth carrying
// login-only credentials while the actual request auth is handled elsewhere
// (or not at all).
func TestLogin_SubstitutesCredentialsInBody(t *testing.T) {
	t.Parallel()

	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "sid-value"})
			w.WriteHeader(http.StatusOK)
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &models.CustomServiceConfig{
		Auth: &models.CustomAuthConfig{
			Mode:     "none",
			Username: "admin",
			Password: "hunter2",
		},
		Login: &models.CustomLoginConfig{
			Method:        "POST",
			Path:          "/login",
			ContentType:   "application/x-www-form-urlencoded",
			Body:          "username={{username}}&password={{password}}",
			CaptureCookie: "SID",
			InjectAs:      "cookie",
			InjectName:    "SID",
		},
		Health: &models.CustomHealthConfig{Path: "/health"},
	}

	service := NewGeneralService().(*GeneralService)
	_, code := service.Engine.CheckHealth(context.Background(), server.URL, "", cfg)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
	if gotBody != "username=admin&password=hunter2" {
		t.Fatalf("login body = %q, want %q", gotBody, "username=admin&password=hunter2")
	}
}

// formatStatValue must not silently format a non-numeric string as 0: a
// string that fails strconv.ParseFloat is returned unchanged for every
// numeric format (e.g. Cleanuparr's upTime "0.12:34:56.789").
func TestFormatStatValue_NonNumericStringPassesThrough(t *testing.T) {
	t.Parallel()

	const raw = "0.12:34:56.789"
	val := gjson.Parse(`"` + raw + `"`)

	for _, format := range []string{"bytes", "duration", "percent", "number"} {
		t.Run(format, func(t *testing.T) {
			if got := formatStatValue(val, format); got != raw {
				t.Errorf("formatStatValue(%q, %q) = %q, want %q (unchanged)", raw, format, got, raw)
			}
		})
	}

	// A numeric string must still format normally.
	numeric := gjson.Parse(`"1024"`)
	if got := formatStatValue(numeric, "bytes"); got != "1.0 KiB" {
		t.Errorf("formatStatValue(%q, bytes) = %q, want %q", "1024", got, "1.0 KiB")
	}
}
