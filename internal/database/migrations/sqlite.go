// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"database/sql"

	"github.com/autobrr/dashbrr/pkg/migrator"

	"github.com/dcarbone/zadapters/zstdlog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func SQLiteMigrator(db *sql.DB) error {
	migrate := migrator.NewMigrate(
		db,
		migrator.WithEmbedFS(SchemaMigrations),
		migrator.WithLogger(zstdlog.NewStdLoggerWithLevel(log.With().Str("module", "database-migrations").Logger(), zerolog.DebugLevel)),
	)

	migrate.Add(
		&migrator.Migration{
			Name: "000_base_schema",
			File: "sqlite_schema.sql",
		},
		&migrator.Migration{
			Name: "001_add_service_access_url",
			RunTx: func(db *sql.Tx) error {
				_, err := db.Exec("ALTER TABLE service_configurations ADD COLUMN access_url TEXT")
				if err != nil {
					return err
				}
				return nil
			},
		},
		&migrator.Migration{
			Name: "002_add_ui_collapse_preferences",
			RunTx: func(db *sql.Tx) error {
				_, err := db.Exec(`
					CREATE TABLE IF NOT EXISTS ui_collapse_preferences
					(
						id             INTEGER PRIMARY KEY,
						user_id        INTEGER NOT NULL,
						preference_key TEXT    NOT NULL,
						is_collapsed   BOOLEAN NOT NULL DEFAULT FALSE,
						updated_at     TIMESTAMP NOT NULL,
						UNIQUE(user_id, preference_key)
					)
				`)
				return err
			},
		},
	)

	err := migrate.Migrate()
	if err != nil {
		return err
	}

	return nil
}
