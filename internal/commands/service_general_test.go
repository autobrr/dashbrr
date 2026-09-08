package commands

import (
	"io"
	"os"
	"reflect"
	"testing"
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
