// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestSanitizeServiceConfig_RedactsCustomConfigSecrets(t *testing.T) {
	cfg := models.ServiceConfiguration{
		InstanceID: "general-1",
		APIKey:     "top-secret-api-key",
		Config: &models.CustomServiceConfig{
			Auth: &models.CustomAuthConfig{
				Mode:     "bearer",
				Token:    "top-secret-token",
				Password: "top-secret-password",
			},
			Login: &models.CustomLoginConfig{
				Method: "POST",
				Path:   "/login",
				Body:   `{"password":"top-secret-password"}`,
			},
		},
	}

	sanitized := sanitizeServiceConfig(cfg)

	if sanitized.APIKey != "" {
		t.Fatalf("expected APIKey to be blanked, got %q", sanitized.APIKey)
	}
	if sanitized.Config == nil {
		t.Fatalf("expected sanitized Config to be non-nil")
	}
	if sanitized.Config.Auth == nil {
		t.Fatalf("expected sanitized Config.Auth to be non-nil")
	}
	if sanitized.Config.Auth.Token != "" {
		t.Fatalf("expected auth.token to be redacted, got %q", sanitized.Config.Auth.Token)
	}
	if sanitized.Config.Auth.Password != "" {
		t.Fatalf("expected auth.password to be redacted, got %q", sanitized.Config.Auth.Password)
	}
	if sanitized.Config.Login.Body != "" {
		t.Fatalf("expected login.body to be redacted, got %q", sanitized.Config.Login.Body)
	}
	// Mode is not a secret and should survive redaction.
	if sanitized.Config.Auth.Mode != "bearer" {
		t.Fatalf("expected auth.mode to survive redaction, got %q", sanitized.Config.Auth.Mode)
	}

	// The original passed-by-value config must be untouched (sanitizeServiceConfig
	// takes the struct by value, but Config is a pointer - make sure it copies
	// rather than mutating the caller's data in place).
	if cfg.Config.Auth.Token != "top-secret-token" {
		t.Fatalf("sanitizeServiceConfig must not mutate the original config; auth.token = %q", cfg.Config.Auth.Token)
	}
	if cfg.Config.Login.Body == "" {
		t.Fatalf("sanitizeServiceConfig must not mutate the original config; login.body was cleared")
	}
}

func TestSanitizeServiceConfig_NilConfig(t *testing.T) {
	cfg := models.ServiceConfiguration{InstanceID: "radarr-1", APIKey: "key"}
	sanitized := sanitizeServiceConfig(cfg)
	if sanitized.Config != nil {
		t.Fatalf("expected nil Config to stay nil, got %+v", sanitized.Config)
	}
	if sanitized.APIKey != "" {
		t.Fatalf("expected APIKey to be blanked")
	}
}

func TestMergeCustomConfigSecrets(t *testing.T) {
	stored := &models.CustomServiceConfig{
		Auth: &models.CustomAuthConfig{
			Mode:     "basic",
			Username: "stored-user",
			Password: "stored-password",
			Token:    "stored-token",
		},
		Login: &models.CustomLoginConfig{
			Method: "POST",
			Path:   "/login",
			Body:   "stored-body",
		},
	}

	t.Run("nil incoming preserves stored entirely", func(t *testing.T) {
		got := mergeCustomConfigSecrets(stored, nil)
		if got != stored {
			t.Fatalf("expected the stored pointer back unchanged, got %+v", got)
		}
	})

	t.Run("nil stored returns incoming as-is", func(t *testing.T) {
		incoming := &models.CustomServiceConfig{Auth: &models.CustomAuthConfig{Mode: "none"}}
		got := mergeCustomConfigSecrets(nil, incoming)
		if got != incoming {
			t.Fatalf("expected the incoming pointer back unchanged, got %+v", got)
		}
	})

	t.Run("empty/placeholder secrets fall back to stored values", func(t *testing.T) {
		incoming := &models.CustomServiceConfig{
			Auth: &models.CustomAuthConfig{
				Mode:     "basic",
				Username: "new-user",
				// Password/Token left empty, as Redacted() would emit.
			},
			Login: &models.CustomLoginConfig{
				Method: "POST",
				Path:   "/login",
				// Body left empty, as Redacted() would emit.
			},
		}

		got := mergeCustomConfigSecrets(stored, incoming)

		if got.Auth.Username != "new-user" {
			t.Fatalf("expected non-secret field to take the incoming value, got %q", got.Auth.Username)
		}
		if got.Auth.Password != "stored-password" {
			t.Fatalf("expected auth.password to be preserved from stored, got %q", got.Auth.Password)
		}
		if got.Auth.Token != "stored-token" {
			t.Fatalf("expected auth.token to be preserved from stored, got %q", got.Auth.Token)
		}
		if got.Login.Body != "stored-body" {
			t.Fatalf("expected login.body to be preserved from stored, got %q", got.Login.Body)
		}

		// The stored config itself must not be mutated.
		if stored.Auth.Username != "stored-user" {
			t.Fatalf("mergeCustomConfigSecrets must not mutate stored; auth.username = %q", stored.Auth.Username)
		}
	})

	t.Run("new secret values are taken from incoming", func(t *testing.T) {
		incoming := &models.CustomServiceConfig{
			Auth: &models.CustomAuthConfig{
				Mode:     "bearer",
				Token:    "new-token",
				Password: "new-password",
			},
			Login: &models.CustomLoginConfig{
				Method: "POST",
				Path:   "/login",
				Body:   "new-body",
			},
		}

		got := mergeCustomConfigSecrets(stored, incoming)

		if got.Auth.Token != "new-token" {
			t.Fatalf("expected auth.token to take the new value, got %q", got.Auth.Token)
		}
		if got.Auth.Password != "new-password" {
			t.Fatalf("expected auth.password to take the new value, got %q", got.Auth.Password)
		}
		if got.Login.Body != "new-body" {
			t.Fatalf("expected login.body to take the new value, got %q", got.Login.Body)
		}
	})

	t.Run("incoming without auth/login clears them even though stored had them", func(t *testing.T) {
		incoming := &models.CustomServiceConfig{TimeoutSeconds: 30}
		got := mergeCustomConfigSecrets(stored, incoming)
		if got.Auth != nil {
			t.Fatalf("expected Auth to stay nil when incoming omits it, got %+v", got.Auth)
		}
		if got.Login != nil {
			t.Fatalf("expected Login to stay nil when incoming omits it, got %+v", got.Login)
		}
		if got.TimeoutSeconds != 30 {
			t.Fatalf("expected TimeoutSeconds from incoming, got %d", got.TimeoutSeconds)
		}
	})
}

// TestSettingsHandler_CustomConfigRoundTrip exercises SaveSettings/GetSettings
// through the real handler with a temp SQLite-backed database: a token saved
// on create must come back redacted, and omitting the secret on a follow-up
// update must not clear the stored value.
func TestSettingsHandler_CustomConfigRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	defer func() { _ = store.Close() }()

	handler := NewSettingsHandler(db, store, nil)

	router := gin.New()
	router.GET("/api/settings", handler.GetSettings)
	router.PUT("/api/settings/:instance", handler.SaveSettings)

	instance := "general-1"

	createBody := `{
		"displayName": "My Service",
		"url": "http://example.com",
		"config": {
			"auth": {"mode": "bearer", "token": "super-secret-token"},
			"health": {"path": "/health"}
		}
	}`
	create := httptest.NewRecorder()
	createReq, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/settings/"+instance, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, createReq)

	if create.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d body=%s", create.Code, create.Body.String())
	}

	var created models.ServiceConfiguration
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if created.Config == nil || created.Config.Auth == nil {
		t.Fatalf("expected config.auth in create response, got %+v", created.Config)
	}
	if created.Config.Auth.Token != "" {
		t.Fatalf("expected create response token to be redacted, got %q", created.Config.Auth.Token)
	}

	// Fetching via GET must also come back redacted.
	get := httptest.NewRecorder()
	getReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/settings", nil)
	router.ServeHTTP(get, getReq)

	if get.Code != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d body=%s", get.Code, get.Body.String())
	}

	var all map[string]models.ServiceConfiguration
	if err := json.Unmarshal(get.Body.Bytes(), &all); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}
	got, ok := all[instance]
	if !ok {
		t.Fatalf("expected %q in GET response, got keys %v", instance, all)
	}
	if got.Config == nil || got.Config.Auth == nil || got.Config.Auth.Token != "" {
		t.Fatalf("expected GET config token to be redacted, got %+v", got.Config)
	}

	// Update without re-sending the token (as the UI would, since it only
	// ever sees the redacted form) must preserve the stored secret.
	updateBody := `{
		"displayName": "My Service Renamed",
		"url": "http://example.com",
		"config": {
			"auth": {"mode": "bearer"},
			"health": {"path": "/health"}
		}
	}`
	update := httptest.NewRecorder()
	updateReq, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/settings/"+instance, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(update, updateReq)

	if update.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d body=%s", update.Code, update.Body.String())
	}

	stored, err := db.FindServiceBy(context.Background(), types.FindServiceParams{InstanceID: instance})
	if err != nil {
		t.Fatalf("FindServiceBy: %v", err)
	}
	if stored == nil || stored.Config == nil || stored.Config.Auth == nil {
		t.Fatalf("expected stored config.auth after update, got %+v", stored)
	}
	if stored.Config.Auth.Token != "super-secret-token" {
		t.Fatalf("expected stored token to be preserved across update, got %q", stored.Config.Auth.Token)
	}
	if stored.DisplayName != "My Service Renamed" {
		t.Fatalf("expected display name to be updated, got %q", stored.DisplayName)
	}
}

func TestSettingsHandler_SaveSettings_RejectsInvalidCustomConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	store := cache.NewMemoryStore(context.Background(), t.TempDir())
	defer func() { _ = store.Close() }()

	handler := NewSettingsHandler(db, store, nil)

	router := gin.New()
	router.PUT("/api/settings/:instance", handler.SaveSettings)

	// 9 stats exceeds the max of 8 and should fail Validate().
	body := `{"displayName":"Bad","url":"http://example.com","config":{"stats":[
		{"label":"a","path":"/a"},{"label":"b","path":"/b"},{"label":"c","path":"/c"},
		{"label":"d","path":"/d"},{"label":"e","path":"/e"},{"label":"f","path":"/f"},
		{"label":"g","path":"/g"},{"label":"h","path":"/h"},{"label":"i","path":"/i"}
	]}}`

	rec := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/settings/general-2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid config, got %d body=%s", rec.Code, rec.Body.String())
	}
}
