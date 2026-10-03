// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
)

const (
	EnvConfigPath = "DASHBRR__CONFIG_PATH"
)

// Config represents the main configuration structure
type Config struct {
	Server   ServerConfig    `toml:"server"`
	Database database.Config `toml:"database"`
	Auth     AuthConfig      `toml:"auth"`
	Log      LogConfig       `toml:"log"`
}

// LogConfig holds logging-related configuration
type LogConfig struct {
	Level string `toml:"level" env:"DASHBRR__LOG_LEVEL"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	ListenAddr  string   `toml:"listen_addr" env:"DASHBRR__LISTEN_ADDR"`
	BasePath    BasePath `toml:"base_path" env:"DASHBRR__BASE_PATH"`
	CORSOrigins []string `toml:"cors_origins" env:"DASHBRR__CORS_ORIGINS"`
	CORSHeaders []string `toml:"cors_headers" env:"DASHBRR__CORS_HEADERS"`
	CORSMethods []string `toml:"cors_methods" env:"DASHBRR__CORS_METHODS"`
	CORSMaxAgeH int      `toml:"cors_max_age_hours" env:"DASHBRR__CORS_MAX_AGE_HOURS"`
	CORSCreds   *bool    `toml:"cors_allow_credentials" env:"DASHBRR__CORS_ALLOW_CREDENTIALS"`
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

// IsConfigured reports whether OIDC is enabled. The redirect URL
// is not part of this test: validation rejects a configured OIDC without it.
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
		Database: *database.DefaultConfig(),
		Log:      LogConfig{Level: "info"},
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

// Flags holds the command-line values that override the config file and the
// environment. An empty value means that the user did not set the flag.
type Flags struct {
	ConfigPath string
	DBPath     string
	ListenAddr string
	// ReadOnly stops Load from writing a default config file when the file
	// is missing.
	ReadOnly bool
}

// Load selects the config file, loads it, and applies the flags. The serve
// command and the CLI commands all use it, so they open the same database.
// It returns the path of the config file that it selected.
func Load(flags Flags) (*Config, string, error) {
	path := findConfigFile(flags.ConfigPath)

	cfg, err := loadConfig(path, !flags.ReadOnly)
	if err != nil {
		return nil, path, err
	}

	if flags.DBPath != "" {
		cfg.Database.Path = flags.DBPath
	}
	if flags.ListenAddr != "" {
		cfg.Server.ListenAddr = flags.ListenAddr
	}

	return cfg, path, nil
}

// findConfigFile returns the --config value, else DASHBRR__CONFIG_PATH, else
// config.toml in the user config directory or /config, else ./config.toml.
func findConfigFile(flagPath string) string {
	if path := cmp.Or(flagPath, os.Getenv(EnvConfigPath)); path != "" {
		return path
	}

	var dirs []string
	if dir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(dir, "dashbrr"))
	}
	dirs = append(dirs, "/config")

	for _, dir := range dirs {
		path := filepath.Join(dir, "config.toml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "config.toml"
}

// LoadConfig reads the config file at path and then applies the environment
// variables. When the file is missing, it tries to write a default file there
// and continues with the defaults if that fails. A relative database path from
// the file is relative to the config directory.
func LoadConfig(path string) (*Config, error) {
	return loadConfig(path, true)
}

func loadConfig(path string, writeDefault bool) (*Config, error) {
	config := DefaultConfig()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error resolving config path: %w", err)
	}

	displayPath := shortenPath(absPath)

	data, err := os.ReadFile(absPath)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !writeDefault:
		log.Debug().Str("path", displayPath).Msg("Configuration file not found, using defaults and environment variables")
	case errors.Is(err, fs.ErrNotExist):
		if err := writeDefaultConfig(absPath, config); err != nil {
			log.Warn().Err(err).Str("path", displayPath).Msg("Configuration file not found and could not be created, using defaults and environment variables")
		} else {
			log.Info().Str("path", displayPath).Msg("Configuration file not found, creating with default values")
		}
	case err != nil:
		return nil, fmt.Errorf("error reading config file %s: %w", displayPath, err)
	default:
		if err := toml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("error decoding config file %s: %w", displayPath, err)
		}
		log.Debug().Str("path", displayPath).Msg("Loaded existing configuration file")
	}

	if !filepath.IsAbs(config.Database.Path) {
		config.Database.Path = filepath.Join(filepath.Dir(absPath), config.Database.Path)
	}

	if err := LoadEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	if config.Server.BasePath, err = normalizeBasePath(string(config.Server.BasePath)); err != nil {
		return nil, err
	}

	if config.Auth.OIDC.IsConfigured() && config.Auth.OIDC.RedirectURL == "" {
		return nil, errors.New("OIDC is configured but redirect_url (DASHBRR__OIDC_REDIRECT_URL) is empty: set it to the callback URL, for example https://example.com/dashbrr/api/auth/oidc/callback")
	}

	return config, nil
}

// BasePath is the URL path prefix that dashbrr is served under. It is "" at
// the root of the host, or a path such as "/dashbrr" with no trailing slash.
type BasePath string

// Path puts the base path in front of p, an in-app path that starts with "/".
func (b BasePath) Path(p string) string {
	return string(b) + p
}

// normalizeBasePath gives "" for an empty value or "/". Otherwise, it gives a
// path with a leading slash and no trailing slash. The server writes the value into an
// HTML attribute, so characters that are not safe there are an error.
func normalizeBasePath(raw string) (BasePath, error) {
	p := strings.TrimSpace(raw)
	// A browser reads "//host" as another site, the same as "http://host".
	if strings.Contains(p, "://") || strings.HasPrefix(p, "//") {
		return "", fmt.Errorf("base_path %q is a path, not a URL: use a value such as /dashbrr", raw)
	}
	// A browser also reads "?" and "#" as the start of a query or a fragment, and "\" as "/".
	if strings.ContainsAny(p, "\"'<>` \t\n?#\\") {
		return "", fmt.Errorf("base_path %q has a character that is not permitted (quote, backtick, <, >, ?, #, \\, or white space)", raw)
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return BasePath(p), nil
}

func writeDefaultConfig(path string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadEnvOverrides loads configuration from environment variables
func LoadEnvOverrides(config *Config) error {
	// Server
	if env := os.Getenv("DASHBRR__LISTEN_ADDR"); env != "" {
		config.Server.ListenAddr = env
	}
	if env := os.Getenv("DASHBRR__BASE_PATH"); env != "" {
		config.Server.BasePath = BasePath(env)
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

	config.Database.ApplyEnvOverrides()

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
