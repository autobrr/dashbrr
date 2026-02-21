// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/traefik"
	"github.com/spf13/cobra"
)

var traefikServiceSpec = serviceSpec{
	Use:         "traefik",
	Prefix:      "traefik-",
	DisplayName: "Traefik",
	Short:       "traefik management",
	Long:        "traefik management",
	Example:     "  dashbrr service traefik\n  dashbrr service traefik --help",
	AddExample:  "  dashbrr service traefik add http://localhost:8080 [AUTH-TOKEN]\n  dashbrr service traefik add --help",
	AddArgs:     cobra.MinimumNArgs(1),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		key := ""
		if len(args) > 1 {
			key = args[1]
		}
		return args[0], key, "Traefik", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return traefik.NewTraefikService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceTraefikCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "traefik",
		Short:        traefikServiceSpec.Short,
		Long:         traefikServiceSpec.Long,
		Example:      traefikServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceTraefikListCommand())
	command.AddCommand(ServiceTraefikAddCommand())
	command.AddCommand(ServiceTraefikRemoveCommand())

	return command
}

func ServiceTraefikListCommand() *cobra.Command   { return newServiceListCommand(traefikServiceSpec) }
func ServiceTraefikAddCommand() *cobra.Command    { return newServiceAddCommand(traefikServiceSpec) }
func ServiceTraefikRemoveCommand() *cobra.Command { return newServiceRemoveCommand(traefikServiceSpec) }
