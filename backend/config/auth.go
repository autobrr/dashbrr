// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/types"
)

// AuthConfig holds the OIDC configuration settings
var AuthConfig *types.AuthConfig

// normalizeIssuerURL ensures consistent trailing slash handling for issuer URLs
func normalizeIssuerURL(issuer string) string {
	// Remove any trailing slashes
	return strings.TrimRight(issuer, "/")
}

// LoadAuthConfig loads the OIDC configuration from config file or environment variables
func LoadAuthConfig() error {
	// Try to load from config file first
	configPath := filepath.Join("config", "auth.json")
	if _, err := os.Stat(configPath); err == nil {
		file, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}

		var config types.AuthConfig
		if err := json.Unmarshal(file, &config); err != nil {
			return err
		}

		// Normalize the issuer URL from config file
		config.Issuer = normalizeIssuerURL(config.Issuer)
		AuthConfig = &config
		log.Info().Msg("loaded auth configuration from file")
		return nil
	}

	// Fall back to environment variables
	issuer := normalizeIssuerURL(getEnvOrDefault("OIDC_ISSUER", ""))
	AuthConfig = &types.AuthConfig{
		Issuer:       issuer,
		ClientID:     getEnvOrDefault("OIDC_CLIENT_ID", ""),
		ClientSecret: getEnvOrDefault("OIDC_CLIENT_SECRET", ""),
		RedirectURL:  getEnvOrDefault("OIDC_REDIRECT_URL", "http://localhost:3000/auth/callback"),
	}

	// Validate required fields
	if AuthConfig.Issuer == "" || AuthConfig.ClientID == "" || AuthConfig.ClientSecret == "" {
		return ErrMissingAuthConfig
	}

	log.Info().Msg("loaded auth configuration from environment")
	return nil
}

// SaveAuthConfig saves the current configuration to file
func SaveAuthConfig() error {
	if AuthConfig == nil {
		return ErrMissingAuthConfig
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll("config", 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(AuthConfig, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join("config", "auth.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	log.Info().Msg("saved auth configuration to file")
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
