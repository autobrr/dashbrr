// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import "errors"

var (
	// ErrMissingAuthConfig is returned when required authentication configuration is missing
	ErrMissingAuthConfig = errors.New("missing required authentication configuration")

	// ErrInvalidAuthConfig is returned when authentication configuration is invalid
	ErrInvalidAuthConfig = errors.New("invalid authentication configuration")

	// ErrConfigFileAccess is returned when there's an error accessing the configuration file
	ErrConfigFileAccess = errors.New("error accessing configuration file")
)
