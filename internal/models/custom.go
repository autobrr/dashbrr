// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"errors"
	"fmt"
	"regexp"
)

// actionIDPattern restricts action ids to something safe to use as a DOM id / URL segment.
var actionIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

const maxCustomEntries = 8

// CustomAuthConfig describes how requests to a custom service should be authenticated.
type CustomAuthConfig struct {
	Mode       string `json:"mode"`
	HeaderName string `json:"headerName,omitempty"`
	QueryParam string `json:"queryParam,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"` //nolint:gosec // struct field name, not a hardcoded credential
	Token      string `json:"token,omitempty"`
}

// CustomLoginConfig describes an optional login step run before health/stat/action requests.
type CustomLoginConfig struct {
	Method          string `json:"method,omitempty"`
	Path            string `json:"path,omitempty"`
	ContentType     string `json:"contentType,omitempty"`
	Body            string `json:"body,omitempty"`
	CaptureCookie   string `json:"captureCookie,omitempty"`
	CaptureJSONPath string `json:"captureJSONPath,omitempty"`
	InjectAs        string `json:"injectAs,omitempty"`
	InjectName      string `json:"injectName,omitempty"`
}

// CustomHealthConfig describes how to determine the service's health.
type CustomHealthConfig struct {
	Method      string   `json:"method,omitempty"`
	Path        string   `json:"path"`
	Body        string   `json:"body,omitempty"`
	StatusPath  string   `json:"statusPath,omitempty"`
	OKValues    []string `json:"okValues,omitempty"`
	WarnValues  []string `json:"warnValues,omitempty"`
	VersionPath string   `json:"versionPath,omitempty"`
}

// CustomStatConfig describes a single stat to extract from a response body.
type CustomStatConfig struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Unit   string `json:"unit,omitempty"`
	Format string `json:"format,omitempty"`
}

// CustomActionConfig describes a single user-triggerable action.
type CustomActionConfig struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Method  string `json:"method,omitempty"`
	Path    string `json:"path"`
	Body    string `json:"body,omitempty"`
	Confirm bool   `json:"confirm,omitempty"`
}

// CustomServiceConfig is a user-defined description of a "general" service's
// authentication, health check, stats and actions.
type CustomServiceConfig struct {
	Auth           *CustomAuthConfig    `json:"auth,omitempty"`
	Login          *CustomLoginConfig   `json:"login,omitempty"`
	Health         *CustomHealthConfig  `json:"health,omitempty"`
	Stats          []CustomStatConfig   `json:"stats,omitempty"`
	Actions        []CustomActionConfig `json:"actions,omitempty"`
	TimeoutSeconds int                  `json:"timeoutSeconds,omitempty"`
}

// Validate checks the config for structural correctness and fills in defaults
// (currently just TimeoutSeconds). It does not validate that paths/JSONPaths
// resolve against any particular response - that's a runtime concern.
func (c *CustomServiceConfig) Validate() error {
	if c == nil {
		return nil
	}

	if c.Auth != nil {
		switch c.Auth.Mode {
		case "":
			return errors.New("auth.mode is required when auth is present")
		case "none", "header", "query", "basic", "bearer":
		default:
			return fmt.Errorf("auth.mode %q is invalid", c.Auth.Mode)
		}
	}

	if c.Login != nil {
		switch c.Login.Method {
		case "", "GET", "POST":
		default:
			return fmt.Errorf("login.method %q is invalid", c.Login.Method)
		}
		switch c.Login.InjectAs {
		case "", "header", "query", "cookie", "bearer":
		default:
			return fmt.Errorf("login.injectAs %q is invalid", c.Login.InjectAs)
		}
	}

	if c.Health != nil {
		if c.Health.Path == "" {
			return errors.New("health.path is required when health is present")
		}
		switch c.Health.Method {
		case "", "GET", "POST":
		default:
			return fmt.Errorf("health.method %q is invalid", c.Health.Method)
		}
	}

	if len(c.Stats) > maxCustomEntries {
		return fmt.Errorf("stats: at most %d entries allowed, got %d", maxCustomEntries, len(c.Stats))
	}
	for i, stat := range c.Stats {
		if stat.Label == "" {
			return fmt.Errorf("stats[%d].label is required", i)
		}
		if stat.Path == "" {
			return fmt.Errorf("stats[%d].path is required", i)
		}
		switch stat.Format {
		case "", "number", "bytes", "duration", "percent", "text":
		default:
			return fmt.Errorf("stats[%d].format %q is invalid", i, stat.Format)
		}
	}

	if len(c.Actions) > maxCustomEntries {
		return fmt.Errorf("actions: at most %d entries allowed, got %d", maxCustomEntries, len(c.Actions))
	}
	for i, action := range c.Actions {
		if action.ID == "" {
			return fmt.Errorf("actions[%d].id is required", i)
		}
		if !actionIDPattern.MatchString(action.ID) {
			return fmt.Errorf("actions[%d].id %q is invalid: must match ^[a-z0-9-]+$", i, action.ID)
		}
		if action.Label == "" {
			return fmt.Errorf("actions[%d].label is required", i)
		}
		if action.Path == "" {
			return fmt.Errorf("actions[%d].path is required", i)
		}
		switch action.Method {
		case "", "GET", "POST", "PUT", "DELETE":
		default:
			return fmt.Errorf("actions[%d].method %q is invalid", i, action.Method)
		}
	}

	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 10
	}

	return nil
}

// Redacted returns a copy of the config with secrets blanked out, safe to
// return from the API.
func (c *CustomServiceConfig) Redacted() CustomServiceConfig {
	redacted := *c

	if c.Auth != nil {
		authCopy := *c.Auth
		authCopy.Password = ""
		authCopy.Token = ""
		redacted.Auth = &authCopy
	}

	if c.Login != nil {
		loginCopy := *c.Login
		loginCopy.Body = ""
		redacted.Login = &loginCopy
	}

	return redacted
}
