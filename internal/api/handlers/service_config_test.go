// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/autobrr/dashbrr/internal/models"
)

func TestRequireServiceConfig(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()

	service := &models.ServiceConfiguration{
		InstanceID:  "radarr-1",
		DisplayName: "Radarr",
		URL:         "https://radarr.example.com",
		APIKey:      "abc123",
	}
	if err := db.CreateService(ctx, service); err != nil {
		t.Fatalf("create service: %v", err)
	}

	got, err := requireServiceConfig(ctx, db, "radarr-1", "radarr")
	if err != nil {
		t.Fatalf("requireServiceConfig returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("requireServiceConfig returned nil config")
	}
	if got.InstanceID != "radarr-1" {
		t.Fatalf("InstanceID = %q, want %q", got.InstanceID, "radarr-1")
	}
}

func TestRequireServiceConfig_NotConfigured(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()

	if _, err := requireServiceConfig(ctx, db, "sonarr-1", "sonarr"); !errors.Is(err, ErrServiceNotConfigured) {
		t.Fatalf("expected ErrServiceNotConfigured, got %v", err)
	}

	service := &models.ServiceConfiguration{
		InstanceID:  "sonarr-1",
		DisplayName: "Sonarr",
		URL:         "",
		APIKey:      "abc123",
	}
	if err := db.CreateService(ctx, service); err != nil {
		t.Fatalf("create service: %v", err)
	}

	if _, err := requireServiceConfig(ctx, db, "sonarr-1", "sonarr"); !errors.Is(err, ErrServiceNotConfigured) {
		t.Fatalf("expected ErrServiceNotConfigured for empty URL, got %v", err)
	}
}

func TestRequireServiceConfigLegacy_NotConfigured(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()

	if _, err := requireServiceConfigLegacy(ctx, db, "autobrr-1"); !errors.Is(err, ErrServiceNotConfigured) {
		t.Fatalf("expected ErrServiceNotConfigured, got %v", err)
	}
}
