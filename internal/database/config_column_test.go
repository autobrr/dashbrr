// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

func fullTestCustomServiceConfig() *models.CustomServiceConfig {
	return &models.CustomServiceConfig{
		Auth: &models.CustomAuthConfig{
			Mode:  "bearer",
			Token: "super-secret-token",
		},
		Health: &models.CustomHealthConfig{
			Method:     "GET",
			Path:       "/health",
			StatusPath: "status",
			OKValues:   []string{"ok", "healthy"},
		},
		Stats: []models.CustomStatConfig{
			{Label: "Queue", Path: "queue.length", Unit: "items", Format: "number"},
		},
		Actions: []models.CustomActionConfig{
			{ID: "restart", Label: "Restart", Method: "POST", Path: "/restart"},
		},
		TimeoutSeconds: 15,
	}
}

// TestConfigColumnRoundTrip verifies that a ServiceConfiguration with a
// non-nil Config survives a create/read round trip on a fresh database.
func TestConfigColumnRoundTrip(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	cfg := fullTestCustomServiceConfig()
	service := &models.ServiceConfiguration{
		InstanceID:  "custom-service-1",
		DisplayName: "Custom Service",
		URL:         "http://localhost:9000",
		Config:      cfg,
	}

	if err := db.CreateService(ctx, service); err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	retrieved, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "custom-service-1"})
	if err != nil {
		t.Fatalf("FindServiceBy failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected to find service, got nil")
	}
	if retrieved.Config == nil {
		t.Fatal("expected Config to round-trip as non-nil")
	}

	want, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal expected config: %v", err)
	}
	got, err := json.Marshal(retrieved.Config)
	if err != nil {
		t.Fatalf("failed to marshal retrieved config: %v", err)
	}
	if string(want) != string(got) {
		t.Errorf("Config round-trip mismatch:\nwant: %s\ngot:  %s", want, got)
	}

	// A service created without a config must read back as nil, not a
	// zero-value struct.
	plain := &models.ServiceConfiguration{
		InstanceID:  "plain-service-1",
		DisplayName: "Plain Service",
		URL:         "http://localhost:9001",
	}
	if err := db.CreateService(ctx, plain); err != nil {
		t.Fatalf("CreateService (no config) failed: %v", err)
	}

	retrievedPlain, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "plain-service-1"})
	if err != nil {
		t.Fatalf("FindServiceBy (no config) failed: %v", err)
	}
	if retrievedPlain == nil {
		t.Fatal("expected to find plain service, got nil")
	}
	if retrievedPlain.Config != nil {
		t.Errorf("expected nil Config for a service created without one, got %+v", retrievedPlain.Config)
	}
}

// legacySQLiteSchema is the service_configurations/users/ui_collapse_preferences
// DDL as it existed before the config column was introduced, applied directly
// (bypassing the migrator) to simulate a database created by an older release.
const legacySQLiteSchema = `
CREATE TABLE IF NOT EXISTS users
(
    id            INTEGER PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMP   NOT NULL,
    updated_at    TIMESTAMP   NOT NULL
);

CREATE TABLE IF NOT EXISTS service_configurations
(
    id           INTEGER PRIMARY KEY,
    instance_id  TEXT UNIQUE NOT NULL,
    display_name TEXT        NOT NULL,
    url          TEXT,
    api_key      TEXT,
    access_url   TEXT
);

CREATE TABLE IF NOT EXISTS ui_collapse_preferences
(
    id             INTEGER PRIMARY KEY,
    user_id        INTEGER NOT NULL,
    preference_key TEXT    NOT NULL,
    is_collapsed   BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at     TIMESTAMP NOT NULL,
    UNIQUE(user_id, preference_key)
);
`

// TestConfigColumnLegacyUpgrade verifies that opening a pre-existing SQLite
// database that predates the config column (no schema_migrations bookkeeping
// at all, i.e. the DDL was applied directly) through the normal database
// initializer adds the column in place, with no data loss and no manual step.
func TestConfigColumnLegacyUpgrade(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy.db")

	legacyCtx := context.Background()

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite db: %v", err)
	}
	if _, err := raw.ExecContext(legacyCtx, legacySQLiteSchema); err != nil {
		raw.Close()
		t.Fatalf("failed to apply legacy schema: %v", err)
	}
	if _, err := raw.ExecContext(
		legacyCtx,
		`INSERT INTO service_configurations (instance_id, display_name, url, api_key, access_url) VALUES (?, ?, ?, ?, ?)`,
		"preexisting-service", "Preexisting Service", "http://localhost:9002", "key", "",
	); err != nil {
		raw.Close()
		t.Fatalf("failed to seed legacy row: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("failed to close raw sqlite db: %v", err)
	}

	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", dbPath)

	config := NewConfig()
	db, err := InitDBWithConfig(config)
	if err != nil {
		t.Fatalf("InitDBWithConfig failed to upgrade legacy database: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(legacyCtx, `PRAGMA table_info(service_configurations)`)
	if err != nil {
		t.Fatalf("failed to inspect table info: %v", err)
	}
	var hasConfig bool
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			t.Fatalf("failed to scan table_info row: %v", err)
		}
		if name == "config" {
			hasConfig = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info iteration error: %v", err)
	}
	if !hasConfig {
		t.Fatal("expected config column to be added to the legacy database")
	}

	ctx := legacyCtx

	// The pre-existing row must survive the upgrade untouched.
	preexisting, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "preexisting-service"})
	if err != nil {
		t.Fatalf("FindServiceBy for preexisting row failed: %v", err)
	}
	if preexisting == nil {
		t.Fatal("expected preexisting row to survive the upgrade")
	}
	if preexisting.Config != nil {
		t.Errorf("expected preexisting row's Config to be nil, got %+v", preexisting.Config)
	}
	if preexisting.URL != "http://localhost:9002" {
		t.Errorf("expected preexisting row's URL to be preserved, got %q", preexisting.URL)
	}

	// And the new column must actually work end to end.
	cfg := &models.CustomServiceConfig{
		Health: &models.CustomHealthConfig{Path: "/healthz"},
	}
	service := &models.ServiceConfiguration{
		InstanceID:  "legacy-upgrade-service",
		DisplayName: "Legacy Upgrade Service",
		URL:         "http://localhost:9003",
		Config:      cfg,
	}
	if err := db.CreateService(ctx, service); err != nil {
		t.Fatalf("CreateService on upgraded database failed: %v", err)
	}

	retrieved, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "legacy-upgrade-service"})
	if err != nil {
		t.Fatalf("FindServiceBy on upgraded database failed: %v", err)
	}
	if retrieved == nil || retrieved.Config == nil {
		t.Fatal("expected config to round-trip on the upgraded legacy database")
	}
	if retrieved.Config.Health == nil || retrieved.Config.Health.Path != "/healthz" {
		t.Errorf("unexpected config after round-trip: %+v", retrieved.Config)
	}
}
