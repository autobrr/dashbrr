// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"os"
	"path/filepath"
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
	if cfg.Auth.OIDC.IsConfigured() {
		t.Error("unrelated TOML settings loaded with the database DSN")
	}
}

func TestPostgresConfigKeepsDefaultsAndEmptyPassword(t *testing.T) {
	t.Setenv("DASHBRR__DB_PASSWORD", "")

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

func TestHasRequiredEnvVarsWithDatabaseDSN(t *testing.T) {
	t.Setenv("DASHBRR__LISTEN_ADDR", ":8080")
	t.Setenv("DASHBRR__DB_TYPE", "postgres")
	t.Setenv("DASHBRR__DB_DSN", "postgres://db.example/dashbrr?sslmode=require")
	t.Setenv("DASHBRR__DB_HOST", "")
	t.Setenv("DASHBRR__DB_PORT", "")
	t.Setenv("DASHBRR__DB_USER", "")
	t.Setenv("DASHBRR__DB_PASSWORD", "")
	t.Setenv("DASHBRR__DB_NAME", "")

	if !HasRequiredEnvVars() {
		t.Error("HasRequiredEnvVars() = false, want true for PostgreSQL DSN")
	}
}

func TestCompleteSQLiteEnvironmentIgnoresConfigFile(t *testing.T) {
	t.Setenv("DASHBRR__LISTEN_ADDR", ":8080")
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", "/data/env.db")

	cfg, err := LoadConfig(writeConfig(t, "not valid toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Database.Path; got != "/data/env.db" {
		t.Errorf("database path = %q, want the value from the environment", got)
	}
}

func TestCompletePostgresEnvironmentIgnoresTOMLWithoutDSN(t *testing.T) {
	t.Setenv("DASHBRR__LISTEN_ADDR", ":8080")
	t.Setenv("DASHBRR__DB_TYPE", "postgres")
	t.Setenv("DASHBRR__DB_HOST", "env-db.example")
	t.Setenv("DASHBRR__DB_PORT", "5432")
	t.Setenv("DASHBRR__DB_USER", "env-user")
	t.Setenv("DASHBRR__DB_PASSWORD", "env-password")
	t.Setenv("DASHBRR__DB_NAME", "env-database")

	cfg, err := LoadConfig(writeConfig(t, oidcTOML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.OIDC.IsConfigured() {
		t.Error("OIDC config loaded from TOML in a complete environment-only setup")
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
		{"complete", OIDCConfig{Issuer: "i", ClientID: "c", ClientSecret: "s"}, true},
		{"no redirect url is still complete", OIDCConfig{Issuer: "i", ClientID: "c", ClientSecret: "s"}, true},
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
