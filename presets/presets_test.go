package presets_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/autobrr/dashbrr/internal/models"
)

// allowedTopLevelKeys mirrors the CustomServiceConfig JSON schema in the
// models package. Kept as a structural check alongside
// TestPresetsValidateThroughProductionModel below - unlike that test, this
// one flags a stray/misspelled top-level key, which decoding into the real
// Go type (with the production decoder's default lenient unmarshalling)
// would otherwise silently ignore.
var allowedTopLevelKeys = map[string]bool{
	"auth":           true,
	"login":          true,
	"health":         true,
	"stats":          true,
	"actions":        true,
	"timeoutSeconds": true,
}

var actionIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// presetFiles lists every *.json file in the presets directory.
func presetFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read presets directory: %v", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".json" {
			files = append(files, e.Name())
		}
	}

	if len(files) == 0 {
		t.Fatal("no preset JSON files found in presets directory")
	}

	sort.Strings(files)
	return files
}

// loadPreset reads and parses a preset file into a generic key->raw map so
// each test can inspect only the parts of the schema it cares about.
func loadPreset(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s is not valid JSON: %v", name, err)
	}

	return raw
}

func TestPresetsAreValidJSON(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			loadPreset(t, name)
		})
	}
}

func TestPresetsOnlyAllowedTopLevelKeys(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			raw := loadPreset(t, name)

			for key := range raw {
				if !allowedTopLevelKeys[key] {
					t.Errorf("%s: unexpected top-level key %q", name, key)
				}
			}
		})
	}
}

func TestPresetsStatsAndActionsLimits(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			raw := loadPreset(t, name)

			if rawStats, ok := raw["stats"]; ok {
				var stats []json.RawMessage
				if err := json.Unmarshal(rawStats, &stats); err != nil {
					t.Fatalf("%s: stats is not an array: %v", name, err)
				}
				if len(stats) > 8 {
					t.Errorf("%s: has %d stats, max is 8", name, len(stats))
				}
			}

			if rawActions, ok := raw["actions"]; ok {
				var actions []struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(rawActions, &actions); err != nil {
					t.Fatalf("%s: actions is not an array: %v", name, err)
				}
				if len(actions) > 8 {
					t.Errorf("%s: has %d actions, max is 8", name, len(actions))
				}
				for _, a := range actions {
					if !actionIDPattern.MatchString(a.ID) {
						t.Errorf("%s: action id %q does not match %s", name, a.ID, actionIDPattern.String())
					}
				}
			}
		})
	}
}

// TestPresetsValidateThroughProductionModel decodes each preset the same
// way production code does (plain json.Unmarshal into
// models.CustomServiceConfig, no DisallowUnknownFields) and runs it through
// the real cfg.Validate(). This catches required-nested-field gaps (e.g.
// "auth": {} missing mode, or a "stats" entry missing label/path) that the
// hand-rolled top-level-key/limits checks above never enforced, and which
// would otherwise only surface when a user imports the preset via
// `--config presets/...` or the UI's Import JSON.
func TestPresetsValidateThroughProductionModel(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}

			var cfg models.CustomServiceConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("%s: failed to unmarshal into models.CustomServiceConfig: %v", name, err)
			}

			if err := cfg.Validate(); err != nil {
				t.Errorf("%s: failed production validation: %v", name, err)
			}
		})
	}
}

// TestPresetsStrictDecodeRejectsUnknownFields decodes every shipped preset
// with json.Decoder.DisallowUnknownFields, catching a misspelled/stray key
// ANYWHERE in the document - including nested one level or more down (e.g.
// "health": {"okValeus": [...]}) - that TestPresetsOnlyAllowedTopLevelKeys
// only ever checked at the top level, and that
// TestPresetsValidateThroughProductionModel can't catch either: a plain
// json.Unmarshal (what production actually uses, and what that test
// deliberately mirrors per the bot's own guidance not to add
// DisallowUnknownFields there) silently drops unknown fields at every
// nesting level instead of erroring.
//
// This is test-only strictness for repo-shipped presets, not a claim about
// what dashbrr accepts at runtime - it exists purely to catch a shipped
// preset with a nested typo (a field that silently does nothing, e.g. an
// "okValeus" that never matches health.okValues) before it reaches a user.
// It is deliberately stricter than the production decoder.
func TestPresetsStrictDecodeRejectsUnknownFields(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}

			var cfg models.CustomServiceConfig
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&cfg); err != nil {
				t.Errorf("%s: strict decode found an unknown field (possible typo, incl. nested): %v", name, err)
			}
		})
	}
}

func TestPresetsHaveHealthPath(t *testing.T) {
	for _, name := range presetFiles(t) {
		t.Run(name, func(t *testing.T) {
			raw := loadPreset(t, name)

			rawHealth, ok := raw["health"]
			if !ok {
				t.Fatalf("%s: missing health block", name)
			}

			var health struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(rawHealth, &health); err != nil {
				t.Fatalf("%s: health is not an object: %v", name, err)
			}
			if health.Path == "" {
				t.Errorf("%s: health.path is empty", name)
			}
		})
	}
}
