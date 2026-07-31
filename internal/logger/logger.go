// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the global logger with colored output
func Init() {
	colors := map[string]string{
		"trace": "\033[36m", // Cyan
		"debug": "\033[33m", // Yellow
		"info":  "\033[34m", // Blue
		"warn":  "\033[33m", // Yellow
		"error": "\033[31m", // Red
		"fatal": "\033[35m", // Magenta
		"panic": "\033[35m", // Magenta
	}

	output := zerolog.ConsoleWriter{
		Out:     os.Stdout,
		NoColor: false,
		FormatLevel: func(i any) string {
			level, ok := i.(string)
			if !ok {
				return "???"
			}
			color := colors[level]
			if color == "" {
				color = "\033[37m" // Default to white
			}
			return color + strings.ToUpper(level) + "\033[0m"
		},
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	// Set the level from the environment first, so that early logs use it.
	// LoadEnvOverrides calls SetLevel again with the value from the config file.
	SetLevel(os.Getenv("DASHBRR__LOG_LEVEL"))
}

// SetLevel sets the global log level. If the value is empty or unknown, SetLevel uses info.
func SetLevel(level string) {
	parsed, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil || parsed == zerolog.NoLevel {
		if level != "" {
			log.Warn().Str("level", level).Msg("Unknown log level, dashbrr uses info instead")
		}
		parsed = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsed)
}
