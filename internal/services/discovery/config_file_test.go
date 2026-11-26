// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportConfig_RejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "url without scheme",
			yaml: `services:
  nzbget:
    - url: http://nzbget-a:6789
      apikey: key
    - url: admin:secret@nzbget-b:6789
      apikey: key
`,
			wantErr: "services.nzbget item 2 url: invalid service URL: must start with http:// or https://",
		},
		{
			name: "access url without scheme",
			yaml: `services:
  nzbget:
    - url: http://nzbget:6789
      access_url: nzbget.example.test
      apikey: key
`,
			wantErr: "services.nzbget item 1 access_url: invalid service URL: must start with http:// or https://",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "services.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			services, err := ImportConfig(path)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if services != nil {
				t.Fatalf("services = %+v, want none", services)
			}
		})
	}
}

func TestImportConfig_TrimsURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.yaml")
	data := "services:\n  nzbget:\n    - url: \"http://nzbget:6789 \"\n      access_url: \" https://nzbget.example.test\"\n      apikey: key\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	services, err := ImportConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 || services[0].URL != "http://nzbget:6789" || services[0].AccessURL != "https://nzbget.example.test" {
		t.Fatalf("services = %+v, want one with trimmed URLs", services)
	}
}
