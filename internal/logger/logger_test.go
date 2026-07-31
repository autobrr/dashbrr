// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package logger

import (
	"testing"

	"github.com/rs/zerolog"
)

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
