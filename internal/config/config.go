// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/logger"
)

const (
	EnvConfigPath = "DASHBRR__CONFIG_PATH"
)

// Config represents the main configuration structure
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Auth     AuthConfig     `toml:"auth"`
	Log      LogConfig      `toml:"log"`
}

// LogConfig holds logging-related configuration
type LogConfig struct {
	Level string `toml:"level" env:"DASHBRR__LOG_LEVEL"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	ListenAddr  string   `toml:"listen_addr" env:"DASHBRR__LISTEN_ADDR"`
	CORSOrigins []string `toml:"cors_origins" env:"DASHBRR__CORS_ORIGINS"`
	CORSHeaders []string `toml:"cors_headers" env:"DASHBRR__CORS_HEADERS"`
	CORSMethods []string `toml:"cors_methods" env:"DASHBRR__CORS_METHODS"`
	CORSMaxAgeH int      `toml:"cors_max_age_hours" env:"DASHBRR__CORS_MAX_AGE_HOURS"`
	CORSCreds   *bool    `toml:"cors_allow_credentials" env:"DASHBRR__CORS_ALLOW_CREDENTIALS"`
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Type     string `toml:"type" env:"DASHBRR__DB_TYPE"`
	Path     string `toml:"path" env:"DASHBRR__DB_PATH"`
	Host     string `toml:"host" env:"DASHBRR__DB_HOST"`
	Port     int    `toml:"port" env:"DASHBRR__DB_PORT"`
	User     string `toml:"user" env:"DASHBRR__DB_USER"`
	Password string `toml:"password" env:"DASHBRR__DB_PASSWORD"`
	Name     string `toml:"name" env:"DASHBRR__DB_NAME"`
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	OIDC OIDCConfig `toml:"oidc"`
}

// OIDCConfig holds OIDC-specific configuration
type OIDCConfig struct {
	Issuer       string `toml:"issuer" env:"DASHBRR__OIDC_ISSUER"`
	ClientID     string `toml:"client_id" env:"DASHBRR__OIDC_CLIENT_ID"`
	ClientSecret string `toml:"client_secret" env:"DASHBRR__OIDC_CLIENT_SECRET"`
	RedirectURL  string `toml:"redirect_url" env:"DASHBRR__OIDC_REDIRECT_URL"`
}

// IsConfigured reports whether OIDC has the three values that it needs. The
// redirect URL has a default, so it is not part of this test.
func (c OIDCConfig) IsConfigured() bool {
	return c.Issuer != "" && c.ClientID != "" && c.ClientSecret != ""
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr: ":8080",
			// Keep empty by default: same-origin deployments don't need CORS.
			// When set, CORS will reflect only these origins and can allow credentials.
			CORSOrigins: nil,
			// Defaults (can be overridden via env/config)
			CORSMaxAgeH: 12,
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: "./data/dashbrr.db",
		},
		Log: LogConfig{Level: "info"},
	}
}

// shortenPath replaces the user's home directory with ~ for display purposes
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(filepath.Clean(path), home, "~", 1)
}

// HasRequiredEnvVars checks if all required environment variables are set
func HasRequiredEnvVars() bool {
	// Check server config
	if os.Getenv("DASHBRR__LISTEN_ADDR") == "" {
		return false
	}

	// Check database config - either SQLite or PostgreSQL must be configured
	dbType := os.Getenv("DASHBRR__DB_TYPE")
	if dbType == "" {
		return false
	}

	switch dbType {
	case "sqlite":
		if os.Getenv("DASHBRR__DB_PATH") == "" {
			return false
		}
	case "postgres":
		requiredVars := []string{
			"DASHBRR__DB_HOST",
			"DASHBRR__DB_PORT",
			"DASHBRR__DB_USER",
			"DASHBRR__DB_PASSWORD",
			"DASHBRR__DB_NAME",
		}
		for _, v := range requiredVars {
			if os.Getenv(v) == "" {
				return false
			}
		}
	default:
		return false
	}

	return true
}

// LoadConfig loads the configuration from environment variables or TOML file
func LoadConfig(path string) (*Config, error) {
	config := &Config{}

	// If all required environment variables are set, use them directly
	if HasRequiredEnvVars() {
		if err := LoadEnvOverrides(config); err != nil {
			return nil, fmt.Errorf("error loading environment variables: %w", err)
		}
		return config, nil
	}

	// Otherwise try to load from config file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error resolving config path: %w", err)
	}

	displayPath := shortenPath(absPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			config = DefaultConfig()

			// Ensure directory exists
			if dir := filepath.Dir(absPath); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return nil, fmt.Errorf("error creating config directory %s: %w", shortenPath(dir), err)
				}
			}

			// Marshal config to TOML
			data, err := toml.Marshal(config)
			if err != nil {
				return nil, fmt.Errorf("error encoding default config: %w", err)
			}

			// Write config file
			if err := os.WriteFile(absPath, data, 0644); err != nil {
				return nil, fmt.Errorf("error writing default config to %s: %w", displayPath, err)
			}
			log.Info().Str("path", displayPath).Msg("Configuration file not found, creating with default values")
		} else {
			return nil, fmt.Errorf("error reading config file %s: %w", displayPath, err)
		}
	} else {
		// Parse existing config file
		if err := toml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("error decoding config file %s: %w", displayPath, err)
		}
		log.Debug().Str("path", displayPath).Msg("Loaded existing configuration file")
	}

	// Override with any environment variables that are set
	if err := LoadEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	return config, nil
}

// LoadEnvOverrides loads configuration from environment variables
func LoadEnvOverrides(config *Config) error {
	// Server
	if env := os.Getenv("DASHBRR__LISTEN_ADDR"); env != "" {
		config.Server.ListenAddr = env
	}
	if env := os.Getenv("DASHBRR__CORS_ORIGINS"); env != "" {
		// comma-separated list (e.g. "http://localhost:3000,https://dash.example.com")
		parts := strings.Split(env, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			o := strings.TrimSpace(p)
			if o != "" {
				origins = append(origins, o)
			}
		}
		config.Server.CORSOrigins = origins
	}
	if env := os.Getenv("DASHBRR__CORS_HEADERS"); env != "" {
		parts := strings.Split(env, ",")
		h := make([]string, 0, len(parts))
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				h = append(h, v)
			}
		}
		config.Server.CORSHeaders = h
	}
	if env := os.Getenv("DASHBRR__CORS_METHODS"); env != "" {
		parts := strings.Split(env, ",")
		m := make([]string, 0, len(parts))
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				m = append(m, v)
			}
		}
		config.Server.CORSMethods = m
	}
	if env := os.Getenv("DASHBRR__CORS_MAX_AGE_HOURS"); env != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(env)); err == nil && n > 0 {
			config.Server.CORSMaxAgeH = n
		}
	}
	if env := os.Getenv("DASHBRR__CORS_ALLOW_CREDENTIALS"); env != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(env)); err == nil {
			config.Server.CORSCreds = &b
		}
	}

	// Database
	if env := os.Getenv("DASHBRR__DB_TYPE"); env != "" {
		config.Database.Type = env
	}
	if env := os.Getenv("DASHBRR__DB_PATH"); env != "" {
		config.Database.Path = env
	}
	if env := os.Getenv("DASHBRR__DB_HOST"); env != "" {
		config.Database.Host = env
	}
	if env := os.Getenv("DASHBRR__DB_PORT"); env != "" {
		if port, err := strconv.Atoi(env); err == nil {
			config.Database.Port = port
		}
	}
	if env := os.Getenv("DASHBRR__DB_USER"); env != "" {
		config.Database.User = env
	}
	if env := os.Getenv("DASHBRR__DB_PASSWORD"); env != "" {
		config.Database.Password = env
	}
	if env := os.Getenv("DASHBRR__DB_NAME"); env != "" {
		config.Database.Name = env
	}

	// Log
	if env := os.Getenv("DASHBRR__LOG_LEVEL"); env != "" {
		config.Log.Level = env
	}

	// Auth OIDC
	if env := os.Getenv("DASHBRR__OIDC_ISSUER"); env != "" {
		config.Auth.OIDC.Issuer = env
	}
	if env := os.Getenv("DASHBRR__OIDC_CLIENT_ID"); env != "" {
		config.Auth.OIDC.ClientID = env
	}
	if env := os.Getenv("DASHBRR__OIDC_CLIENT_SECRET"); env != "" {
		config.Auth.OIDC.ClientSecret = env
	}
	if env := os.Getenv("DASHBRR__OIDC_REDIRECT_URL"); env != "" {
		config.Auth.OIDC.RedirectURL = env
	}

	warnRenamedOIDCVars()

	// Every config path goes through this function, so it is the one place that applies the level.
	logger.SetLevel(config.Log.Level)

	return nil
}

// warnRenamedOIDCVars reports OIDC variables that still use the old, unprefixed
// name. Dashbrr ignores those names, and without this warning an upgrade
// disables OIDC login with nothing in the log to explain it.
func warnRenamedOIDCVars() {
	for _, name := range []string{"OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL"} {
		if os.Getenv(name) != "" && os.Getenv("DASHBRR__"+name) == "" {
			log.Warn().
				Str("old", name).
				Str("new", "DASHBRR__"+name).
				Msg("Ignored OIDC variable with the old name, rename it")
		}
	}
}
