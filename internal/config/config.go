// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
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

// LoadConfig loads the configuration from a TOML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	config := &Config{}
	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %w", err)
	}

	// Override with environment variables if they exist
	if err := loadEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	return config, nil
}

// loadEnvOverrides checks for environment variables and overrides config values
func loadEnvOverrides(config *Config) error {
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
