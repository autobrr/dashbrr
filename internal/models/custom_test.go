// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import "testing"

func fullCustomServiceConfig() *CustomServiceConfig {
	return &CustomServiceConfig{
		Auth: &CustomAuthConfig{
			Mode:  "bearer",
			Token: "super-secret-token",
		},
		Login: &CustomLoginConfig{
			Method:        "POST",
			Path:          "/login",
			ContentType:   "application/json",
			Body:          `{"user":"admin","pass":"hunter2"}`,
			CaptureCookie: "session",
			InjectAs:      "cookie",
			InjectName:    "session",
		},
		Health: &CustomHealthConfig{
			Method:      "GET",
			Path:        "/health",
			StatusPath:  "status",
			OKValues:    []string{"ok", "healthy"},
			WarnValues:  []string{"degraded"},
			VersionPath: "version",
		},
		Stats: []CustomStatConfig{
			{Label: "Queue", Path: "queue.length", Unit: "items", Format: "number"},
			{Label: "Uptime", Path: "uptime", Format: "duration"},
		},
		Actions: []CustomActionConfig{
			{ID: "restart", Label: "Restart", Method: "POST", Path: "/restart", Confirm: true},
		},
		TimeoutSeconds: 15,
	}
}

func TestCustomServiceConfigValidate_FullDefinition(t *testing.T) {
	cfg := fullCustomServiceConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() on a full definition returned error: %v", err)
	}
}

func TestCustomServiceConfigValidate_DefaultsTimeout(t *testing.T) {
	cfg := &CustomServiceConfig{
		Health: &CustomHealthConfig{Path: "/health"},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if cfg.TimeoutSeconds != 10 {
		t.Errorf("expected TimeoutSeconds to default to 10, got %d", cfg.TimeoutSeconds)
	}
}

func TestCustomServiceConfigValidate_RejectsTooManyStats(t *testing.T) {
	cfg := &CustomServiceConfig{}
	for i := 0; i < 9; i++ {
		cfg.Stats = append(cfg.Stats, CustomStatConfig{Label: "stat", Path: "path"})
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() with 9 stats returned nil error, want a rejection")
	}
}

func TestCustomServiceConfigValidate_RejectsTooManyActions(t *testing.T) {
	cfg := &CustomServiceConfig{}
	for i := 0; i < 9; i++ {
		cfg.Actions = append(cfg.Actions, CustomActionConfig{ID: "action", Label: "Action", Path: "/x"})
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() with 9 actions returned nil error, want a rejection")
	}
}

func TestCustomServiceConfigValidate_RejectsBadActionID(t *testing.T) {
	cfg := &CustomServiceConfig{
		Actions: []CustomActionConfig{
			{ID: "Not Valid!", Label: "Bad Action", Path: "/x"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() with a bad action id returned nil error, want a rejection")
	}
}

func TestCustomServiceConfigValidate_RejectsMissingAuthMode(t *testing.T) {
	cfg := &CustomServiceConfig{
		Auth: &CustomAuthConfig{Token: "abc"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() with auth present but no mode returned nil error, want a rejection")
	}
}

func TestCustomServiceConfigValidate_RejectsMissingHealthPath(t *testing.T) {
	cfg := &CustomServiceConfig{
		Health: &CustomHealthConfig{Method: "GET"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() with health present but no path returned nil error, want a rejection")
	}
}

func TestCustomServiceConfigRedacted_BlanksSecrets(t *testing.T) {
	cfg := fullCustomServiceConfig()

	redacted := cfg.Redacted()

	if redacted.Auth == nil {
		t.Fatal("expected Auth to remain non-nil after Redacted()")
	}
	if redacted.Auth.Token != "" {
		t.Errorf("expected Auth.Token to be blanked, got %q", redacted.Auth.Token)
	}
	if redacted.Auth.Password != "" {
		t.Errorf("expected Auth.Password to be blanked, got %q", redacted.Auth.Password)
	}
	if redacted.Login == nil {
		t.Fatal("expected Login to remain non-nil after Redacted()")
	}
	if redacted.Login.Body != "" {
		t.Errorf("expected Login.Body to be blanked, got %q", redacted.Login.Body)
	}

	// Original must be untouched.
	if cfg.Auth.Token == "" {
		t.Error("Redacted() must not mutate the original config's Auth.Token")
	}
	if cfg.Login.Body == "" {
		t.Error("Redacted() must not mutate the original config's Login.Body")
	}

	// Non-secret fields should be preserved.
	if redacted.Login.Path != cfg.Login.Path {
		t.Errorf("expected Login.Path to be preserved, got %q want %q", redacted.Login.Path, cfg.Login.Path)
	}
	if len(redacted.Stats) != len(cfg.Stats) {
		t.Errorf("expected Stats to be preserved, got %d want %d", len(redacted.Stats), len(cfg.Stats))
	}
}
