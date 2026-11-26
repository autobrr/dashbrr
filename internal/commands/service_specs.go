// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/catalog"
)

func onlineOnly(health models.ServiceHealth) bool {
	return health.Status == "online"
}

var serviceSpecs = map[string]serviceSpec{
	"autobrr":     {DisplayName: "Autobrr", AddUsage: []string{"http://localhost:7474 <API-KEY>"}, HealthOK: onlineOnly},
	"radarr":      {DisplayName: "Radarr", AddUsage: []string{"http://localhost:7878 <API-KEY>"}},
	"sonarr":      {DisplayName: "Sonarr", AddUsage: []string{"http://localhost:8989 <API-KEY>"}},
	"whisparr":    {DisplayName: "Whisparr", AddUsage: []string{"http://localhost:6969 <API-KEY>"}},
	"lidarr":      {DisplayName: "Lidarr", AddUsage: []string{"http://localhost:8686 <API-KEY>"}},
	"readarr":     {DisplayName: "Readarr", AddUsage: []string{"http://localhost:8787 <API-KEY>"}},
	"bazarr":      {DisplayName: "Bazarr", AddUsage: []string{"http://localhost:6767 <API-KEY>"}},
	"sabnzbd":     {DisplayName: "SABnzbd", AddUsage: []string{"http://localhost:8080 <API-KEY>"}},
	"nzbget":      {DisplayName: "NZBGet", AddUsage: []string{"http://localhost:6789 <CONTROL-PASSWORD>"}},
	"prowlarr":    {DisplayName: "Prowlarr", AddUsage: []string{"http://localhost:9696 <API-KEY>"}},
	"overseerr":   {DisplayName: "Overseerr", AddUsage: []string{"http://localhost:5055 <API-KEY>"}},
	"plex":        {DisplayName: "Plex", AddUsage: []string{"http://localhost:32400 <TOKEN>"}},
	"jellyfin":    {DisplayName: "Jellyfin", AddUsage: []string{"http://localhost:8096 <API-KEY>"}},
	"emby":        {DisplayName: "Emby", AddUsage: []string{"http://localhost:8096 <API-KEY>"}},
	"uptimekuma":  {DisplayName: "Uptime Kuma", AddUsage: []string{"http://localhost:3001 <API-KEY>"}},
	"maintainerr": {DisplayName: "Maintainerr", AddUsage: []string{"http://localhost:6246 <API-KEY>"}},
	"qui":         {DisplayName: "Qui", AddUsage: []string{"http://localhost:7476 <API-KEY>"}},
	"traefik": {
		DisplayName: "Traefik",
		AddUsage:    []string{"http://localhost:8080 [AUTH-TOKEN]"},
		AddArgs:     cobra.MinimumNArgs(1),
		ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
			key := ""
			if len(args) > 1 {
				key = args[1]
			}
			return args[0], key, "Traefik", nil
		},
	},
	"tailscale": {
		DisplayName: "Tailscale",
		AddUsage:    []string{"<API-KEY>"},
		AddArgs:     cobra.MinimumNArgs(1),
		ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
			return "https://api.tailscale.com", args[0], "Tailscale", nil
		},
	},
	"general": {
		Use:         "generic",
		DisplayName: "General",
		AddUsage:    []string{"[url] [name]", "[url] [name] [apiKey]"},
		ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
			serviceURL = args[0]
			displayName = args[1]
			if len(args) >= 3 {
				apiKey = args[2]
			}
			if displayName == "" {
				return "", "", "", errors.New("name is required")
			}
			return serviceURL, apiKey, displayName, nil
		},
		HealthOK: onlineOnly,
	},
}

func serviceSpecFor(t catalog.ServiceType) (serviceSpec, bool) {
	spec, ok := serviceSpecs[t.Name]
	if !ok {
		return serviceSpec{}, false
	}
	spec.Use = cmp.Or(spec.Use, t.Name)
	spec.Prefix = t.Name + "-"
	if spec.AddArgs == nil {
		spec.AddArgs = cobra.MinimumNArgs(2)
	}
	if spec.ParseAdd == nil {
		displayName := spec.DisplayName
		spec.ParseAdd = func(args []string) (string, string, string, error) {
			return args[0], args[1], displayName, nil
		}
	}
	spec.HealthCheck = func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return t.New().CheckHealth(ctx, serviceURL, apiKey)
	}
	return spec, true
}

func newServiceTypeCommand(spec serviceSpec) *cobra.Command {
	command := &cobra.Command{
		Use:          spec.Use,
		Short:        spec.Use + " management",
		Long:         spec.Use + " management",
		Example:      fmt.Sprintf("  dashbrr service %s\n  dashbrr service %s --help", spec.Use, spec.Use),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}
	command.AddCommand(newServiceListCommand(spec))
	command.AddCommand(newServiceAddCommand(spec))
	command.AddCommand(newServiceRemoveCommand(spec))
	return command
}
