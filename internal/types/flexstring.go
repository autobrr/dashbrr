// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import (
	"bytes"
	"encoding/json"
)

// FlexString unmarshals from either a JSON string or a JSON number and stores
// the value as a string. Marshalling needs no custom hook: as a named string
// type it always encodes back out as a JSON string.
//
// SABnzbd changed several queue fields (e.g. noofslots, noofslots_total) from
// JSON strings to JSON numbers in newer releases; dashbrr and its web frontend
// still treat them as strings. This type keeps the upstream parse working while
// preserving the string contract the frontend expects.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	// JSON number (int or float) -> canonical string form
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexString(n)
	return nil
}
