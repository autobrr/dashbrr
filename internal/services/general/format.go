// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// formatStatValue renders a gjson value per one of the stat.Format values
// D2 supports: bytes, duration, percent, number, text (default).
//
// For the numeric formats, a string value that doesn't actually parse as a
// number (e.g. Cleanuparr's upTime "0.12:34:56.789") is returned unchanged
// rather than silently formatted as 0 - misconfiguring the format is a
// display-time distraction, not a reason to fabricate a value.
func formatStatValue(val gjson.Result, format string) string {
	switch format {
	case "bytes", "duration", "percent", "number":
		if val.Type == gjson.String {
			if _, err := strconv.ParseFloat(val.String(), 64); err != nil {
				return val.String()
			}
		}
	}

	switch format {
	case "bytes":
		return formatBytes(val.Float())
	case "duration":
		return formatDuration(val.Float())
	case "percent":
		return fmt.Sprintf("%.1f%%", val.Float())
	case "number":
		return formatNumber(val.Float())
	default: // "text", ""
		return val.String()
	}
}

// formatBytes renders n bytes using IEC (1024-based) units with 1 decimal.
func formatBytes(n float64) string {
	const unit = 1024.0
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

	neg := n < 0
	if neg {
		n = -n
	}

	value := n
	idx := 0
	for value >= unit && idx < len(units)-1 {
		value /= unit
		idx++
	}

	sign := ""
	if neg {
		sign = "-"
	}

	return fmt.Sprintf("%s%.1f %s", sign, value, units[idx])
}

// formatDuration renders a number of seconds as "1d 2h 3m", dropping units
// that are zero (unless everything is zero, in which case it shows seconds).
func formatDuration(totalSeconds float64) string {
	total := int64(totalSeconds)
	if total < 0 {
		total = 0
	}

	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}

// formatNumber renders v with thousands separators, e.g. 1234567 -> "1,234,567".
//
// The whole value is rounded to 2 decimal places once, up front, and only
// then split into integer/fraction parts. Rounding after the split (the
// previous approach) could carry a fraction like 0.999 up to "1.00" without
// that carry propagating into intPart, corrupting the output (e.g. 1.999
// rendered as "11." instead of "2").
func formatNumber(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}

	rounded := math.Round(v*100) / 100

	intPart := int64(rounded)
	frac := rounded - float64(intPart)

	out := addThousandsSeparators(strconv.FormatInt(intPart, 10))
	if frac > 0.0009 {
		fracStr := strings.TrimPrefix(fmt.Sprintf("%.2f", frac), "0")
		fracStr = strings.TrimRight(fracStr, "0")
		if fracStr != "." {
			out += fracStr
		}
	}

	if neg {
		out = "-" + out
	}

	return out
}

func addThousandsSeparators(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}

	var groups []string
	for n > 3 {
		groups = append([]string{digits[n-3:]}, groups...)
		digits = digits[:n-3]
		n = len(digits)
	}
	groups = append([]string{digits}, groups...)

	return strings.Join(groups, ",")
}
