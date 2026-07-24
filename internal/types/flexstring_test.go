// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

import (
	"encoding/json"
	"testing"
)

func TestFlexStringUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  FlexString
	}{
		{"quoted string", `"42"`, "42"},
		{"bare integer", `42`, "42"},
		{"bare float", `4.5`, "4.5"},
		{"empty string", `""`, ""},
		{"null", `null`, ""},
		{"text string", `"queued"`, "queued"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FlexString
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal(%s) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	var f FlexString
	if err := json.Unmarshal([]byte(`{"not":"a scalar"}`), &f); err == nil {
		t.Error("Unmarshal(object) expected error, got nil")
	}
}

func TestFlexStringMarshal(t *testing.T) {
	// Always marshals back as a JSON string, preserving the frontend contract.
	b, err := json.Marshal(FlexString("42"))
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if string(b) != `"42"` {
		t.Errorf("Marshal = %s, want %q", b, `"42"`)
	}
}

func TestFlexStringQueueRoundTrip(t *testing.T) {
	// SABnzbd >=4.5.5 sends these as bare numbers; older versions as strings.
	type queue struct {
		NoOfSlots      FlexString `json:"noofslots"`
		NoOfSlotsTotal FlexString `json:"noofslots_total"`
	}
	for _, input := range []string{
		`{"noofslots":3,"noofslots_total":7}`,
		`{"noofslots":"3","noofslots_total":"7"}`,
	} {
		var q queue
		if err := json.Unmarshal([]byte(input), &q); err != nil {
			t.Fatalf("Unmarshal(%s) error: %v", input, err)
		}
		if q.NoOfSlots != "3" || q.NoOfSlotsTotal != "7" {
			t.Errorf("Unmarshal(%s) = %+v, want 3/7", input, q)
		}
		out, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if string(out) != `{"noofslots":"3","noofslots_total":"7"}` {
			t.Errorf("Marshal = %s, want string-typed fields", out)
		}
	}
}
