// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/jellyfin"
)

var embyServiceSpec = serviceSpec{
	Use:         "emby",
	Prefix:      "emby-",
	DisplayName: "Emby",
	Short:       "emby management",
	Long:        "emby management",
	Example:     "  dashbrr service emby\n  dashbrr service emby --help",
	AddExample:  "  dashbrr service emby add http://localhost:8096 <API-KEY>\n  dashbrr service emby add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Emby", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return jellyfin.NewEmbyService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceEmbyCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "emby",
		Short:        embyServiceSpec.Short,
		Long:         embyServiceSpec.Long,
		Example:      embyServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceEmbyListCommand())
	command.AddCommand(ServiceEmbyAddCommand())
	command.AddCommand(ServiceEmbyRemoveCommand())

	return command
}

func ServiceEmbyListCommand() *cobra.Command { return newServiceListCommand(embyServiceSpec) }
func ServiceEmbyAddCommand() *cobra.Command  { return newServiceAddCommand(embyServiceSpec) }
func ServiceEmbyRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(embyServiceSpec)
}
