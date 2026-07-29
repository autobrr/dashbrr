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

func TestSabnzbdQueueEnvelopeSlotCounts(t *testing.T) {
	// Regression test for the parse failure in #90. SABnzbd >=4.5.5 emits
	// noofslots/noofslots_total as bare JSON numbers, older releases as strings;
	// both must decode into SabnzbdQueue, and both must re-encode as JSON strings
	// because web/src/types/service.ts declares them string and SabnzbdStats.tsx
	// calls .trim() on the value.
	tests := []struct {
		name  string
		input string
	}{
		{
			"numeric counts (SABnzbd >=4.5.5)",
			`{"queue":{"status":"Downloading","noofslots":3,"noofslots_total":7,"have_warnings":"0","slots":[]}}`,
		},
		{
			"string counts (legacy SABnzbd)",
			`{"queue":{"status":"Downloading","noofslots":"3","noofslots_total":"7","have_warnings":"0","slots":[]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var envelope SabnzbdQueueEnvelope
			if err := json.Unmarshal([]byte(tt.input), &envelope); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if envelope.Queue.NoOfSlots != "3" || envelope.Queue.NoOfSlotsTotal != "7" {
				t.Errorf(
					"noofslots/noofslots_total = %q/%q, want 3/7",
					envelope.Queue.NoOfSlots,
					envelope.Queue.NoOfSlotsTotal,
				)
			}

			// Decoding the re-encoded queue into plain string fields fails if the
			// counts ever serialize back out as JSON numbers.
			out, err := json.Marshal(envelope.Queue)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			var wire struct {
				NoOfSlots      string `json:"noofslots"`
				NoOfSlotsTotal string `json:"noofslots_total"`
			}
			if err := json.Unmarshal(out, &wire); err != nil {
				t.Fatalf("re-encoded queue is not string-typed for the frontend: %v", err)
			}
			if wire.NoOfSlots != "3" || wire.NoOfSlotsTotal != "7" {
				t.Errorf("re-encoded = %q/%q, want 3/7", wire.NoOfSlots, wire.NoOfSlotsTotal)
			}
		})
	}
}

func TestSabnzbdQueueSlotPriority(t *testing.T) {
	// SABnzbd builds slot["priority"] as INTERFACE_PRIORITIES.get(nzo.priority,
	// NORMAL_PRIORITY). The mapped values are strings but the fallback is the
	// integer 0, so a queued job holding DEFAULT (-100) or PAUSED (-2) priority --
	// both settable via mode=queue&name=priority -- emits a bare number here.
	tests := []struct {
		name  string
		input string
		want  FlexString
	}{
		{"mapped priority (string)", `{"queue":{"slots":[{"nzo_id":"a","priority":"Normal"}]}}`, "Normal"},
		{"unmapped priority (numeric fallback)", `{"queue":{"slots":[{"nzo_id":"a","priority":0}]}}`, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var envelope SabnzbdQueueEnvelope
			if err := json.Unmarshal([]byte(tt.input), &envelope); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if len(envelope.Queue.Slots) != 1 {
				t.Fatalf("slots len = %d, want 1", len(envelope.Queue.Slots))
			}
			if got := envelope.Queue.Slots[0].Priority; got != tt.want {
				t.Errorf("priority = %q, want %q", got, tt.want)
			}
		})
	}
}
