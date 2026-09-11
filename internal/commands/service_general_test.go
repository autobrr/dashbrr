package commands

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/autobrr/dashbrr/internal/database"
)

func TestParseGeneralStatFlag(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"path only", "Queue=queue.length", "Queue|queue.length||", false},
		{"with unit", "Queue=queue.length:items", "Queue|queue.length|items|", false},
		{"with unit and format", "Uptime=uptime:seconds:duration", "Uptime|uptime|seconds|duration", false},
		{"missing equals", "Queue-queue.length", "", true},
		{"empty label", "=queue.length", "", true},
		{"empty path", "Queue=", "", true},
		{"too many segments", "Queue=queue.length:items:number:extra", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGeneralStatFlag(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.raw, err)
			}
			gotJoined := got.Label + "|" + got.Path + "|" + got.Unit + "|" + got.Format
			if gotJoined != tt.want {
				t.Fatalf("parseGeneralStatFlag(%q) = %+v, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestGeneralDefinitionFlags_BuildConfig_Empty(t *testing.T) {
	flags := &generalDefinitionFlags{}
	cfg, err := flags.buildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config when no flags are set, got %+v", cfg)
	}
}

func TestGeneralDefinitionFlags_BuildConfig_FromFlags(t *testing.T) {
	flags := &generalDefinitionFlags{
		authMode:    "header",
		headerName:  "X-Api-Key",
		healthPath:  "/health",
		statusPath:  "status",
		ok:          []string{"ok", "healthy"},
		warn:        []string{"degraded"},
		versionPath: "version",
		stats:       []string{"Queue=queue.length:items:number"},
	}

	cfg, err := flags.buildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected a non-nil config")
	}

	if cfg.Auth == nil || cfg.Auth.Mode != "header" || cfg.Auth.HeaderName != "X-Api-Key" {
		t.Fatalf("unexpected Auth: %+v", cfg.Auth)
	}
	if cfg.Health == nil || cfg.Health.Path != "/health" || cfg.Health.StatusPath != "status" || cfg.Health.VersionPath != "version" {
		t.Fatalf("unexpected Health: %+v", cfg.Health)
	}
	if !reflect.DeepEqual(cfg.Health.OKValues, []string{"ok", "healthy"}) {
		t.Fatalf("unexpected OKValues: %+v", cfg.Health.OKValues)
	}
	if !reflect.DeepEqual(cfg.Health.WarnValues, []string{"degraded"}) {
		t.Fatalf("unexpected WarnValues: %+v", cfg.Health.WarnValues)
	}
	if len(cfg.Stats) != 1 || cfg.Stats[0].Label != "Queue" || cfg.Stats[0].Path != "queue.length" ||
		cfg.Stats[0].Unit != "items" || cfg.Stats[0].Format != "number" {
		t.Fatalf("unexpected Stats: %+v", cfg.Stats)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected a valid config, got: %v", err)
	}
}

func TestGeneralDefinitionFlags_BuildConfig_InvalidStat(t *testing.T) {
	flags := &generalDefinitionFlags{stats: []string{"no-equals-sign"}}
	if _, err := flags.buildConfig(); err == nil {
		t.Fatal("expected an error for a malformed --stat value")
	}
}

func TestGeneralDefinitionFlags_BuildConfig_ConfigFileOverridesFlags(t *testing.T) {
	path := writeTempGeneralConfigFile(t, `{"health": {"path": "/from-file"}}`)

	flags := &generalDefinitionFlags{
		configFile: path,
		healthPath: "/from-flag", // must be ignored: --config is authoritative
	}

	cfg, err := flags.buildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Health == nil || cfg.Health.Path != "/from-file" {
		t.Fatalf("expected config to come from --config file, got %+v", cfg)
	}
}

// runGeneralTest must surface health.Message and record it as a failure
// (via Error) when the service reports a non-ok status, even though
// FetchStats itself never runs/errors - regression for the bug where only
// a FetchStats error was treated as a test failure.
func TestRunGeneralTest_UnhealthyStatus_SurfacesMessageAndError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"offline","message":"database unreachable"}`))
	}))
	defer server.Close()

	result := runGeneralTest(context.Background(), server.URL, "", nil)

	if result.Status != "offline" {
		t.Fatalf("Status = %q, want %q", result.Status, "offline")
	}
	if result.Message != "database unreachable" {
		t.Fatalf("Message = %q, want %q", result.Message, "database unreachable")
	}
	if result.Error == "" {
		t.Fatal("expected Error to be set for an offline status, got empty")
	}
	if !strings.Contains(result.Error, "database unreachable") {
		t.Fatalf("Error = %q, want it to contain the health message", result.Error)
	}
}

// A "warning" status must not be treated as a test failure - only offline/
// error statuses should fail the command.
func TestRunGeneralTest_WarningStatus_IsNotAFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"warning","message":"degraded"}`))
	}))
	defer server.Close()

	result := runGeneralTest(context.Background(), server.URL, "", nil)

	if result.Status != "warning" {
		t.Fatalf("Status = %q, want %q", result.Status, "warning")
	}
	if result.Error != "" {
		t.Fatalf("expected no Error for a warning status, got %q", result.Error)
	}
}

// ServiceGeneralTestCommand must exit non-zero when the target service
// reports an offline/unhealthy status, not just on a FetchStats error - a
// calling script has no other way to detect the failure.
func TestServiceGeneralTestCommand_UnhealthyStatus_ExitsNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"offline","message":"database unreachable"}`))
	}))
	defer server.Close()

	cmd := ServiceGeneralTestCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{server.URL})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-zero exit for an offline service status")
	}
	if !strings.Contains(err.Error(), "database unreachable") {
		t.Fatalf("error = %q, want it to contain the health message", err.Error())
	}
}

func writeTempGeneralConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := t.TempDir() + "/config.json"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}
	return path
}

// An invalid --auth-mode fails cfg.Validate() before any database or
// network access, so this exercises the CLI's non-zero-exit contract
// without needing a live database or upstream service.
func TestServiceGeneralAddCommand_RejectsInvalidDefinition(t *testing.T) {
	cmd := ServiceGeneralAddCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"http://localhost:9000", "Test Service", "--auth-mode=bogus"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a non-zero exit for an invalid --auth-mode")
	}
}

func TestServiceGeneralTestCommand_RejectsInvalidDefinition(t *testing.T) {
	cmd := ServiceGeneralTestCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"http://localhost:9000", "--stat=malformed"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a non-zero exit for a malformed --stat flag")
	}
}

func TestServiceGeneralTestCommand_RejectsInvalidURL(t *testing.T) {
	cmd := ServiceGeneralTestCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"not-a-url"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a non-zero exit for an invalid URL")
	}
}

// Regression test: `add` must gate connectivity with the config-aware
// engine, not the nil-config legacy check. The fixture 401s the legacy
// probe path ("/", no credential header) and only answers 200 on the
// configured health path with the configured header credential - so this
// fails against the old "always GET / with apiKey as Bearer" gate and
// passes once the gate is config-aware.
func TestServiceGeneralAddCommand_WithConfig_UsesConfiguredHealthCheck(t *testing.T) {
	t.Chdir(t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" && r.Header.Get("X-Api-Key") == "secret-key" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	cmd := ServiceGeneralAddCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		server.URL, "Custom Service", "secret-key",
		"--auth-mode=header",
		"--header-name=X-Api-Key",
		"--health-path=/health",
		"--status-path=status",
		"--ok=ok",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected the add to succeed against the configured health check, got: %v", err)
	}

	db, err := database.InitDB("./data/dashbrr.db")
	if err != nil {
		t.Fatalf("failed to open persisted database: %v", err)
	}
	defer db.Close()

	services, err := db.GetAllServices(context.Background())
	if err != nil {
		t.Fatalf("GetAllServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected exactly 1 persisted service, got %d", len(services))
	}
	if services[0].URL != server.URL {
		t.Fatalf("URL = %q, want %q", services[0].URL, server.URL)
	}
	if services[0].Config == nil || services[0].Config.Health == nil || services[0].Config.Health.Path != "/health" {
		t.Fatalf("expected the configured health path to be persisted, got %+v", services[0].Config)
	}
}

// Plain `add` with no flags/--config must keep using the legacy nil-config
// check (GET base URL, apiKey as Bearer) unchanged.
func TestServiceGeneralAddCommand_LegacyNoConfig_StillWorks(t *testing.T) {
	t.Chdir(t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cmd := ServiceGeneralAddCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{server.URL, "Legacy Service"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected the legacy no-config add to succeed, got: %v", err)
	}

	db, err := database.InitDB("./data/dashbrr.db")
	if err != nil {
		t.Fatalf("failed to open persisted database: %v", err)
	}
	defer db.Close()

	services, err := db.GetAllServices(context.Background())
	if err != nil {
		t.Fatalf("GetAllServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected exactly 1 persisted service, got %d", len(services))
	}
	if services[0].Config != nil {
		t.Fatalf("expected no config for a plain legacy add, got %+v", services[0].Config)
	}
}
