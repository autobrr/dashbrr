package presets_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// allowedTopLevelKeys mirrors the CustomServiceConfig JSON schema in the
// models package. This package is deliberately standalone (no imports from
// the rest of the module), so the schema is checked structurally rather
// than by unmarshalling into the real Go type.
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
