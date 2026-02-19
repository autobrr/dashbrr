// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

func findServiceConfig(ctx context.Context, db *database.DB, instanceID string) (*models.ServiceConfiguration, error) {
	return db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceID})
}

func requireServiceConfig(ctx context.Context, db *database.DB, instanceID, serviceType string) (*models.ServiceConfiguration, error) {
	cfg, err := findServiceConfig(ctx, db, instanceID)
	if err != nil {
		return nil, err
	}

	if cfg == nil || cfg.URL == "" {
		return nil, NewServiceNotConfigured(serviceType)
	}

	return cfg, nil
}

func requireServiceConfigLegacy(ctx context.Context, db *database.DB, instanceID string) (*models.ServiceConfiguration, error) {
	cfg, err := findServiceConfig(ctx, db, instanceID)
	if err != nil {
		return nil, err
	}

	if cfg == nil || cfg.URL == "" {
		return nil, ErrServiceNotConfigured
	}

	return cfg, nil
}
