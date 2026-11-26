// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/catalog"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func TestServiceCommandTree(t *testing.T) {
	var b strings.Builder
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		fmt.Fprintf(&b, "## %s\n", cmd.CommandPath())
		fmt.Fprintf(&b, "use: %q\naliases: %q\nshort: %q\nlong: %q\nexample: %q\nsilence usage: %v\n", cmd.Use, cmd.Aliases, cmd.Short, cmd.Long, cmd.Example, cmd.SilenceUsage)
		if cmd.Args != nil {
			accepted := []int{}
			for n := range 4 {
				if cmd.Args(cmd, make([]string, n)) == nil {
					accepted = append(accepted, n)
				}
			}
			fmt.Fprintf(&b, "accepts arg counts: %v\n", accepted)
		}
		if f := cmd.Flags().Lookup("dry-run"); f != nil {
			fmt.Fprintf(&b, "flag dry-run: %q\n", f.Usage)
		}
		b.WriteString("\n")
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(ServiceCommand())

	golden := filepath.Join("testdata", "service_commands.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Fatalf("the dashbrr service commands changed; if that is on purpose, run go test ./internal/commands -run TestServiceCommandTree -update\n--- got ---\n%s", b.String())
	}
}

func TestServiceSpecParseAddAndHealthOK(t *testing.T) {
	warning := models.ServiceHealth{Status: "warning"}
	tests := []struct {
		serviceType string
		args        []string
		wantURL     string
		wantKey     string
		wantName    string
		warningOK   bool
	}{
		{"autobrr", []string{"http://a.test", "key"}, "http://a.test", "key", "Autobrr", false},
		{"bazarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Bazarr", true},
		{"emby", []string{"http://a.test", "key"}, "http://a.test", "key", "Emby", true},
		{"general", []string{"http://a.test", "My App"}, "http://a.test", "", "My App", false},
		{"general", []string{"http://a.test", "My App", "key"}, "http://a.test", "key", "My App", false},
		{"jellyfin", []string{"http://a.test", "key"}, "http://a.test", "key", "Jellyfin", true},
		{"lidarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Lidarr", true},
		{"maintainerr", []string{"http://a.test", "key"}, "http://a.test", "key", "Maintainerr", true},
		{"nzbget", []string{"http://a.test", "key"}, "http://a.test", "key", "NZBGet", true},
		{"overseerr", []string{"http://a.test", "key"}, "http://a.test", "key", "Overseerr", true},
		{"plex", []string{"http://a.test", "key"}, "http://a.test", "key", "Plex", true},
		{"prowlarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Prowlarr", true},
		{"qui", []string{"http://a.test", "key"}, "http://a.test", "key", "Qui", true},
		{"radarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Radarr", true},
		{"readarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Readarr", true},
		{"sabnzbd", []string{"http://a.test", "key"}, "http://a.test", "key", "SABnzbd", true},
		{"sonarr", []string{"http://a.test", "key"}, "http://a.test", "key", "Sonarr", true},
		{"tailscale", []string{"key"}, "https://api.tailscale.com", "key", "Tailscale", true},
		{"traefik", []string{"http://a.test"}, "http://a.test", "", "Traefik", true},
		{"traefik", []string{"http://a.test", "token"}, "http://a.test", "token", "Traefik", true},
		{"uptimekuma", []string{"http://a.test", "key"}, "http://a.test", "key", "Uptime Kuma", true},
		{"whisparr", []string{"http://a.test", "key"}, "http://a.test", "key", "Whisparr", true},
	}
	for _, tt := range tests {
		t.Run(tt.serviceType+"/"+strings.Join(tt.args, ","), func(t *testing.T) {
			spec := specFor(t, tt.serviceType)
			if got := spec.Prefix; got != tt.serviceType+"-" {
				t.Errorf("Prefix = %q, want %q", got, tt.serviceType+"-")
			}
			url, key, name, err := spec.ParseAdd(tt.args)
			if err != nil {
				t.Fatalf("ParseAdd: %v", err)
			}
			if url != tt.wantURL || key != tt.wantKey || name != tt.wantName {
				t.Errorf("ParseAdd = (%q, %q, %q), want (%q, %q, %q)", url, key, name, tt.wantURL, tt.wantKey, tt.wantName)
			}
			ok := defaultHealthOK(warning)
			if spec.HealthOK != nil {
				ok = spec.HealthOK(warning)
			}
			if ok != tt.warningOK {
				t.Errorf("a warning health counts as ok = %v, want %v", ok, tt.warningOK)
			}
		})
	}
}

func TestGeneralServiceSpecNeedsName(t *testing.T) {
	if _, _, _, err := specFor(t, "general").ParseAdd([]string{"http://a.test", ""}); err == nil {
		t.Fatal("ParseAdd accepted an empty name")
	}
}

func specFor(t *testing.T, serviceType string) serviceSpec {
	t.Helper()
	st, ok := catalog.Lookup(serviceType)
	if !ok {
		t.Fatalf("no service type %q", serviceType)
	}
	spec, ok := serviceSpecFor(st)
	if !ok {
		t.Fatalf("no CLI spec for %q", serviceType)
	}
	return spec
}

func TestEveryServiceTypeHasCLICommands(t *testing.T) {
	for _, st := range catalog.All() {
		if _, ok := serviceSpecFor(st); !ok {
			t.Errorf("service type %q has no entry in serviceSpecs in internal/commands/service_specs.go", st.Name)
		}
	}
	for name := range serviceSpecs {
		if _, ok := catalog.Lookup(name); !ok {
			t.Errorf("serviceSpecs has %q, which is not a service type in internal/services/catalog", name)
		}
	}
}
