// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package logger

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestInitWritesToStderr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })

	t.Setenv("DASHBRR__LOG_LEVEL", "")
	origStderr, origLogger, origLevel := os.Stderr, log.Logger, zerolog.GlobalLevel()
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr, log.Logger = origStderr, origLogger
		zerolog.SetGlobalLevel(origLevel)
	})

	// The ConsoleWriter has one output, so a line on stderr is not on stdout.
	Init()
	log.Info().Msg("hello")
	w.Close()

	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "hello") {
		t.Errorf("stderr = %q, want the log line", out)
	}
}

func TestSetLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"WARN", zerolog.WarnLevel},
		{" error ", zerolog.ErrorLevel},
		{"", zerolog.InfoLevel},
		{"nonsense", zerolog.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			SetLevel(tt.input)
			if got := zerolog.GlobalLevel(); got != tt.want {
				t.Errorf("SetLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
