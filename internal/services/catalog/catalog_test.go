// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package catalog_test

import (
	"fmt"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/catalog"
)

func TestRegistryCreatesEveryServiceType(t *testing.T) {
	want := map[string]string{
		"autobrr":     "*autobrr.AutobrrService",
		"bazarr":      "*bazarr.BazarrService",
		"emby":        "*jellyfin.JellyfinService",
		"general":     "*general.GeneralService",
		"jellyfin":    "*jellyfin.JellyfinService",
		"lidarr":      "*lidarr.LidarrService",
		"maintainerr": "*maintainerr.MaintainerrService",
		"netbird":     "*netbird.Service",
		"nzbget":      "*nzbget.NzbgetService",
		"overseerr":   "*overseerr.OverseerrService",
		"plex":        "*plex.PlexService",
		"prowlarr":    "*prowlarr.ProwlarrService",
		"qui":         "*qui.QuiService",
		"radarr":      "*radarr.RadarrService",
		"readarr":     "*readarr.ReadarrService",
		"sabnzbd":     "*sabnzbd.SabnzbdService",
		"sonarr":      "*sonarr.SonarrService",
		"tailscale":   "*tailscale.TailscaleService",
		"traefik":     "*traefik.TraefikService",
		"uptimekuma":  "*uptimekuma.UptimeKumaService",
		"whisparr":    "*whisparr.WhisparrService",
	}
	registry := catalog.Registry{}
	for serviceType, wantType := range want {
		if got := fmt.Sprintf("%T", registry.CreateService(serviceType)); got != wantType {
			t.Errorf("CreateService(%q) = %s, want %s", serviceType, got, wantType)
		}
	}
	if got := fmt.Sprintf("%T", registry.CreateService("Sonarr")); got != want["sonarr"] {
		t.Errorf("CreateService(%q) = %s, want %s", "Sonarr", got, want["sonarr"])
	}
	for _, serviceType := range []string{"", "other", "nonexistent"} {
		if got := registry.CreateService(serviceType); got != nil {
			t.Errorf("CreateService(%q) = %T, want nil", serviceType, got)
		}
	}
}
