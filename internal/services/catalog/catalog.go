// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package catalog

import (
	"slices"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/bazarr"
	"github.com/autobrr/dashbrr/internal/services/general"
	"github.com/autobrr/dashbrr/internal/services/jellyfin"
	"github.com/autobrr/dashbrr/internal/services/lidarr"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/netbird"
	"github.com/autobrr/dashbrr/internal/services/nzbget"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/qui"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/readarr"
	"github.com/autobrr/dashbrr/internal/services/sabnzbd"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/services/traefik"
	"github.com/autobrr/dashbrr/internal/services/uptimekuma"
	"github.com/autobrr/dashbrr/internal/services/whisparr"
)

type ServiceType struct {
	Name string
	New  func() models.ServiceHealthChecker
}

var serviceTypes = []ServiceType{
	{"autobrr", autobrr.NewAutobrrService},
	{"radarr", radarr.NewRadarrService},
	{"sonarr", sonarr.NewSonarrService},
	{"whisparr", whisparr.NewWhisparrService},
	{"lidarr", lidarr.NewLidarrService},
	{"readarr", readarr.NewReadarrService},
	{"bazarr", bazarr.NewBazarrService},
	{"sabnzbd", sabnzbd.NewSabnzbdService},
	{"nzbget", nzbget.NewNzbgetService},
	{"prowlarr", prowlarr.NewProwlarrService},
	{"traefik", traefik.NewTraefikService},
	{"overseerr", overseerr.NewOverseerrService},
	{"plex", plex.NewPlexService},
	{"jellyfin", jellyfin.NewJellyfinService},
	{"emby", jellyfin.NewEmbyService},
	{"uptimekuma", uptimekuma.NewUptimeKumaService},
	{"tailscale", tailscale.NewTailscaleService},
	{"netbird", netbird.NewNetBirdService},
	{"maintainerr", maintainerr.NewMaintainerrService},
	{"qui", qui.NewQuiService},
	{"general", general.NewGeneralService},
}

func All() []ServiceType {
	return slices.Clone(serviceTypes)
}

func Lookup(name string) (ServiceType, bool) {
	name = strings.ToLower(name)
	i := slices.IndexFunc(serviceTypes, func(t ServiceType) bool { return t.Name == name })
	if i < 0 {
		return ServiceType{}, false
	}
	return serviceTypes[i], true
}

type Registry struct{}

func (Registry) CreateService(serviceType string) models.ServiceHealthChecker {
	t, ok := Lookup(serviceType)
	if !ok {
		return nil
	}
	return t.New()
}
