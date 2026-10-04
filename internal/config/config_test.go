// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const oidcTOML = `
[auth.oidc]
issuer = "https://toml.example.com"
client_id = "toml-id"
client_secret = "toml-secret"
redirect_url = "https://toml.example.com/callback"
`

func TestOIDCFromTOML(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, oidcTOML))
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Auth.OIDC.Issuer; got != "https://toml.example.com" {
		t.Errorf("issuer = %q, want the value from the config file", got)
	}
	if !cfg.Auth.OIDC.IsConfigured() {
		t.Error("IsConfigured() = false, want true")
	}
}

func TestDatabaseDSNFromTOML(t *testing.T) {
	dsn := "postgres://db.example/dashbrr?sslmode=verify-full&sslrootcert=/certs/root.crt&sslcert=/certs/client.crt&sslkey=/certs/client.key"
	cfg, err := LoadConfig(writeConfig(t, "[database]\ntype = \"postgres\"\ndsn = \""+dsn+"\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Database.DSN; got != dsn {
		t.Errorf("database DSN = %q, want %q", got, dsn)
	}
}

func TestDatabaseDSNEnvOverridesTOML(t *testing.T) {
	t.Setenv("DASHBRR__DB_DSN", "postgres://env.example/dashbrr?sslmode=require")

	cfg, err := LoadConfig(writeConfig(t, "[database]\ntype = \"postgres\"\ndsn = \"postgres://toml.example/dashbrr?sslmode=disable\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Database.DSN; got != "postgres://env.example/dashbrr?sslmode=require" {
		t.Errorf("database DSN = %q, want the value from the environment", got)
	}
}

func TestDatabaseDSNTakesPriorityOverSeparateEnvironmentFields(t *testing.T) {
	t.Setenv("DASHBRR__LISTEN_ADDR", ":8080")
	t.Setenv("DASHBRR__DB_TYPE", "postgres")
	t.Setenv("DASHBRR__DB_HOST", "env-db.example")
	t.Setenv("DASHBRR__DB_PORT", "5432")
	t.Setenv("DASHBRR__DB_USER", "env-user")
	t.Setenv("DASHBRR__DB_PASSWORD", "env-password")
	t.Setenv("DASHBRR__DB_NAME", "env-database")

	want := "postgres://toml.example/dashbrr?sslmode=require"
	cfg, err := LoadConfig(writeConfig(t, "[database]\ntype = \"postgres\"\ndsn = \""+want+"\"\n"+oidcTOML))
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Database.DSN; got != want {
		t.Errorf("database DSN = %q, want %q", got, want)
	}
}

func TestPostgresConfigKeepsDefaultsAndEmptyPassword(t *testing.T) {
	t.Setenv("DASHBRR__DB_TYPE", "")
	t.Setenv("DASHBRR__DB_PATH", "")
	t.Setenv("DASHBRR__DB_DSN", "")
	t.Setenv("DASHBRR__DB_HOST", "")
	t.Setenv("DASHBRR__DB_PORT", "")
	t.Setenv("DASHBRR__DB_USER", "")
	t.Setenv("DASHBRR__DB_PASSWORD", "")
	t.Setenv("DASHBRR__DB_NAME", "")

	cfg, err := LoadConfig(writeConfig(t, "[database]\ntype = \"postgres\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Database.Host != "localhost" || cfg.Database.Port != 5432 || cfg.Database.User != "dashbrr" || cfg.Database.DBName != "dashbrr" {
		t.Errorf("database defaults not preserved: %#v", cfg.Database)
	}
	if cfg.Database.Password != "" {
		t.Errorf("database password = %q, want explicit empty value", cfg.Database.Password)
	}
}

func TestOIDCEnvOverridesTOML(t *testing.T) {
	t.Setenv("DASHBRR__OIDC_ISSUER", "https://env.example.com")

	cfg, err := LoadConfig(writeConfig(t, oidcTOML))
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Auth.OIDC.Issuer; got != "https://env.example.com" {
		t.Errorf("issuer = %q, want the value from the environment", got)
	}
	if got := cfg.Auth.OIDC.ClientID; got != "toml-id" {
		t.Errorf("client_id = %q, want the value from the config file", got)
	}
}

func TestOIDCIsConfigured(t *testing.T) {
	tests := []struct {
		name string
		oidc OIDCConfig
		want bool
	}{
		{"complete", OIDCConfig{Issuer: "i", ClientID: "c", ClientSecret: "s", RedirectURL: "r"}, true},
		{"no redirect url is still configured", OIDCConfig{Issuer: "i", ClientID: "c", ClientSecret: "s"}, true},
		{"no secret", OIDCConfig{Issuer: "i", ClientID: "c"}, false},
		{"empty", OIDCConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.oidc.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

// isolateConfigSearch keeps the tests away from a real config file in the
// user config directory and from the database environment variables.
// It returns the user config directory.
func isolateConfigSearch(t *testing.T) string {
	t.Helper()
	userConfigDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userConfigDir)
	t.Setenv(EnvConfigPath, "")
	t.Setenv("DASHBRR__DB_PATH", "")
	t.Setenv("DASHBRR__DB_TYPE", "")
	t.Setenv("DASHBRR__LISTEN_ADDR", "")
	t.Chdir(t.TempDir())
	return userConfigDir
}

func TestLoadConfigFilePriority(t *testing.T) {
	userDir := filepath.Join(isolateConfigSearch(t), "dashbrr")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	userPath := filepath.Join(userDir, "config.toml")
	if err := os.WriteFile(userPath, []byte("[server]\nlisten_addr = \":1\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	envPath := writeConfig(t, "[server]\nlisten_addr = \":2\"\n")
	flagPath := writeConfig(t, "[server]\nlisten_addr = \":3\"\n")

	_, path, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if path != userPath {
		t.Errorf("without flag or env, path = %q, want the user config dir file %q", path, userPath)
	}

	t.Setenv(EnvConfigPath, envPath)
	cfg, path, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if path != envPath || cfg.Server.ListenAddr != ":2" {
		t.Errorf("with %s, got path %q listen %q, want %q :2", EnvConfigPath, path, cfg.Server.ListenAddr, envPath)
	}

	cfg, path, err = Load(Flags{ConfigPath: flagPath})
	if err != nil {
		t.Fatal(err)
	}
	if path != flagPath || cfg.Server.ListenAddr != ":3" {
		t.Errorf("with --config, got path %q listen %q, want %q :3", path, cfg.Server.ListenAddr, flagPath)
	}
}

func TestLoadFallsBackToWorkingDirectory(t *testing.T) {
	isolateConfigSearch(t)
	if _, err := os.Stat("/config/config.toml"); err == nil {
		t.Skip("/config has a config file on this host")
	}

	cfg, path, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if path != "config.toml" {
		t.Errorf("path = %q, want config.toml", path)
	}
	wd, _ := os.Getwd()
	if want := filepath.Join(wd, "data", "dashbrr.db"); cfg.Database.Path != want {
		t.Errorf("database path = %q, want %q", cfg.Database.Path, want)
	}
}

func TestLoadDatabasePath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "abs.db")

	tests := []struct {
		name string
		toml string
		env  string
		flag string
		want func(configDir, wd string) string
	}{
		{"default is in config dir", "", "", "", func(c, _ string) string { return filepath.Join(c, "data", "dashbrr.db") }},
		{"relative file value is relative to config dir", "[database]\npath = \"./db/x.db\"\n", "", "", func(c, _ string) string { return filepath.Join(c, "db", "x.db") }},
		{"absolute file value", "[database]\npath = \"" + abs + "\"\n", "", "", func(_, _ string) string { return abs }},
		{"env alone wins over file", "[database]\npath = \"./db/x.db\"\n", "env.db", "", func(_, wd string) string { return filepath.Join(wd, "env.db") }},
		{"flag wins over env", "", "env.db", "flag.db", func(_, wd string) string { return filepath.Join(wd, "flag.db") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateConfigSearch(t)
			t.Setenv("DASHBRR__DB_PATH", tt.env)
			path := writeConfig(t, tt.toml)
			wd, _ := os.Getwd()

			cfg, _, err := Load(Flags{ConfigPath: path, DBPath: tt.flag})
			if err != nil {
				t.Fatal(err)
			}
			got, _ := filepath.Abs(cfg.Database.Path)
			if want := tt.want(filepath.Dir(path), wd); got != want {
				t.Errorf("database path = %q, want %q", got, want)
			}
		})
	}
}

func TestLoadListenAddr(t *testing.T) {
	isolateConfigSearch(t)
	path := writeConfig(t, "[server]\nlisten_addr = \":1\"\n")

	cfg, _, err := Load(Flags{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddr != ":1" {
		t.Errorf("listen = %q, want the config file value", cfg.Server.ListenAddr)
	}

	t.Setenv("DASHBRR__LISTEN_ADDR", ":2")
	cfg, _, err = Load(Flags{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddr != ":2" {
		t.Errorf("listen = %q, want the env value", cfg.Server.ListenAddr)
	}

	// The flag wins even when it holds the default value.
	cfg, _, err = Load(Flags{ConfigPath: path, ListenAddr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("listen = %q, want the flag value", cfg.Server.ListenAddr)
	}
}

func TestLoadMissingConfigInReadOnlyDir(t *testing.T) {
	isolateConfigSearch(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	t.Setenv("DASHBRR__LISTEN_ADDR", ":9")

	path := filepath.Join(dir, "config.toml")
	if f, err := os.Create(path); err == nil {
		f.Close()
		t.Skip("directory is writable, probably running as root")
	}

	cfg, _, err := Load(Flags{ConfigPath: path})
	if err != nil {
		t.Fatalf("Load returned %v, want a warning only", err)
	}
	if cfg.Server.ListenAddr != ":9" {
		t.Errorf("listen = %q, want the env value", cfg.Server.ListenAddr)
	}
	if want := filepath.Join(dir, "data", "dashbrr.db"); cfg.Database.Path != want {
		t.Errorf("database path = %q, want %q", cfg.Database.Path, want)
	}
}

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		in      string
		want    BasePath
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "/", want: ""},
		{in: "dashbrr", want: "/dashbrr"},
		{in: "/dashbrr", want: "/dashbrr"},
		{in: "/dashbrr/", want: "/dashbrr"},
		{in: " /dashbrr ", want: "/dashbrr"},
		{in: "/a/b", want: "/a/b"},
		{in: "/apps/../dashbrr/", want: "/dashbrr"},
		{in: "/./dashbrr", want: "/dashbrr"},
		{in: "http://x/y", wantErr: true},
		{in: `/a"b`, wantErr: true},
		{in: "/a'b", wantErr: true},
		{in: "/a<b>", wantErr: true},
		{in: "/a b", wantErr: true},
		{in: "/dash?x", wantErr: true},
		{in: "/dash#x", wantErr: true},
		{in: `/\example.com`, wantErr: true},
		{in: "//example.com", wantErr: true},
		{in: "/*app", wantErr: true},
		{in: "/:app", wantErr: true},
		{in: "/health/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := normalizeBasePath(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeBasePath(%q) = %q, want an error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeBasePath(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("normalizeBasePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBasePathPath(t *testing.T) {
	tests := []struct {
		name string
		base BasePath
		in   string
		want string
	}{
		{"root redirect at root", "", "/", "/"},
		{"root redirect and cookie path under prefix", "/dashbrr", "/", "/dashbrr/"},
		{"OIDC error redirect at root", "", "/login?error=no_code", "/login?error=no_code"},
		{"OIDC error redirect under prefix", "/dashbrr", "/login?error=no_code", "/dashbrr/login?error=no_code"},
		{"nested prefix", "/a/b", "/login", "/a/b/login"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.base.Path(tt.in); got != tt.want {
				t.Errorf("BasePath(%q).Path(%q) = %q, want %q", tt.base, tt.in, got, tt.want)
			}
		})
	}
}

func TestBasePathFromTOMLAndEnv(t *testing.T) {
	path := writeConfig(t, "[server]\nbase_path = \"dashbrr/\"\n")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.BasePath != "/dashbrr" {
		t.Errorf("base_path from file = %q, want /dashbrr", cfg.Server.BasePath)
	}

	t.Setenv("DASHBRR__BASE_PATH", "/env/")
	cfg, err = LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.BasePath != "/env" {
		t.Errorf("base_path from env = %q, want /env", cfg.Server.BasePath)
	}
}

func TestBasePathRejectsURL(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "[server]\nbase_path = \"https://example.com/dashbrr\"\n"))
	if err == nil || !strings.Contains(err.Error(), "path, not a URL") {
		t.Fatalf("LoadConfig error = %v, want one that says base_path is a path, not a URL", err)
	}
}

func TestOIDCWithoutRedirectURLFails(t *testing.T) {
	body := strings.Replace(oidcTOML, `redirect_url = "https://toml.example.com/callback"`, "", 1)
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "redirect_url") {
		t.Fatalf("LoadConfig error = %v, want one that names redirect_url", err)
	}
}

func TestKubernetesDiscoveryConfig(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, "[discovery.kubernetes]\nenabled = true\nnamespaces = [\"media\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	k := cfg.Discovery.Kubernetes
	if !k.Enabled || len(k.Namespaces) != 1 || k.Namespaces[0] != "media" || k.IntervalMinutes != 5 {
		t.Errorf("from TOML: %+v, want enabled, [media], default interval 5", k)
	}

	t.Setenv("DASHBRR__K8S_DISCOVERY_ENABLED", "false")
	t.Setenv("DASHBRR__K8S_DISCOVERY_NAMESPACES", "media, downloads")
	t.Setenv("DASHBRR__K8S_DISCOVERY_INTERVAL_MINUTES", "10")
	cfg, err = LoadConfig(writeConfig(t, "[discovery.kubernetes]\nenabled = true\n"))
	if err != nil {
		t.Fatal(err)
	}
	k = cfg.Discovery.Kubernetes
	if k.Enabled || len(k.Namespaces) != 2 || k.Namespaces[1] != "downloads" || k.IntervalMinutes != 10 {
		t.Errorf("from env: %+v, want disabled, [media downloads], interval 10", k)
	}
}
