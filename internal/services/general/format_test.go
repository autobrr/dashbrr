// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import "testing"

// Regression test: formatNumber used to round the fractional part alone,
// so a value whose fraction rounds up to 1.00 (e.g. 1.999) corrupted the
// output ("11." instead of "2"). Rounding the whole value once before
// splitting it fixes this.
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"carry_from_fraction", 1.999, "2"},
		{"half_rounds_up", 0.995, "1"},
		{"large_carry", 999.999, "1,000"},
		{"integer", 1234567, "1,234,567"},
		{"simple_fraction", 1234.5, "1,234.5"},
		{"trailing_zero_dropped", 1.50, "1.5"},
		{"tiny_fraction_dropped", 1.0004, "1"},
		{"zero", 0, "0"},
		{"negative_carry", -1.999, "-2"},
		{"negative_simple", -1234.5, "-1,234.5"},
		{"negative_integer", -42, "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNumber(tt.in)
			if got != tt.want {
				t.Fatalf("formatNumber(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
