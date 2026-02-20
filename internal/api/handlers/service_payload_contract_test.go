package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var canonicalServiceMessageKeys = []string{
	"plex_sessions",
	"jellyfin_summary",
	"uptimekuma_summary",
	"overseerr_requests",
	"radarr_queue",
	"lidarr_queue",
	"readarr_queue",
	"sonarr_queue",
	"sonarr_stats",
	"prowlarr_stats",
	"prowlarr_indexers",
	"bazarr_summary",
	"sabnzbd_summary",
	"nzbget_summary",
	"autobrr_stats",
	"autobrr_releases",
	"autobrr_irc_status",
	"maintainerr_collections",
	"tailscale_devices",
	"qui_overview",
}

func canonicalMessageAssignmentPattern() *regexp.Regexp {
	return regexp.MustCompile(
		fmt.Sprintf(`Message:\s+"(?:%s)"`, strings.Join(canonicalServiceMessageKeys, "|")),
	)
}

func TestCanonicalServicePayloadMessagesOnlyInBuilder(t *testing.T) {
	pattern := canonicalMessageAssignmentPattern()
	root := "."

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", root, err)
	}

	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == "service_payload_builders.go" {
			continue
		}

		path := filepath.Join(root, name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		if pattern.Match(b) {
			offenders = append(offenders, path)
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("found canonical payload message assignments outside builder: %v", offenders)
	}
}

func TestCanonicalServicePayloadMessagesDeclaredOnceInBuilder(t *testing.T) {
	builderPath := "service_payload_builders.go"
	b, err := os.ReadFile(builderPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", builderPath, err)
	}
	src := string(b)

	for _, key := range canonicalServiceMessageKeys {
		pattern := regexp.MustCompile(fmt.Sprintf(`Message:\s+"%s"`, regexp.QuoteMeta(key)))
		matches := pattern.FindAllStringIndex(src, -1)
		if len(matches) != 1 {
			t.Fatalf("expected Message %q exactly once in %s, got %d", key, builderPath, len(matches))
		}
	}
}
