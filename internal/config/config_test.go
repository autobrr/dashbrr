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
