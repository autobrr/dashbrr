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
)

const (
	EnvConfigPath = "DASHBRR__CONFIG_PATH"
)

// Config represents the main configuration structure
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Cache    CacheConfig    `toml:"cache"`
	Database DatabaseConfig `toml:"database"`
	Auth     AuthConfig     `toml:"auth"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	ListenAddr string `toml:"listen_addr" env:"DASHBRR__LISTEN_ADDR"`
}

// CacheConfig holds cache-related configuration
type CacheConfig struct {
	Type  string      `toml:"type" env:"CACHE_TYPE"`
	Redis RedisConfig `toml:"redis"`
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	Host string `toml:"host" env:"REDIS_HOST"`
	Port int    `toml:"port" env:"REDIS_PORT"`
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
	Issuer       string `toml:"issuer" env:"OIDC_ISSUER"`
	ClientID     string `toml:"client_id" env:"OIDC_CLIENT_ID"`
	ClientSecret string `toml:"client_secret" env:"OIDC_CLIENT_SECRET"`
	RedirectURL  string `toml:"redirect_url" env:"OIDC_REDIRECT_URL"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr: ":8080",
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: "./data/dashbrr.db",
		},
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

	// Cache
	if env := os.Getenv("CACHE_TYPE"); env != "" {
		config.Cache.Type = env
	}
	if env := os.Getenv("REDIS_HOST"); env != "" {
		config.Cache.Redis.Host = env
	}
	if env := os.Getenv("REDIS_PORT"); env != "" {
		if port, err := strconv.Atoi(env); err == nil {
			config.Cache.Redis.Port = port
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

	// Auth OIDC
	if env := os.Getenv("OIDC_ISSUER"); env != "" {
		config.Auth.OIDC.Issuer = env
	}
	if env := os.Getenv("OIDC_CLIENT_ID"); env != "" {
		config.Auth.OIDC.ClientID = env
	}
	if env := os.Getenv("OIDC_CLIENT_SECRET"); env != "" {
		config.Auth.OIDC.ClientSecret = env
	}
	if env := os.Getenv("OIDC_REDIRECT_URL"); env != "" {
		config.Auth.OIDC.RedirectURL = env
	}

	return nil
}
