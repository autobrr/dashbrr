package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/jellyfin"
	"github.com/spf13/cobra"
)

var jellyfinServiceSpec = serviceSpec{
	Use:         "jellyfin",
	Prefix:      "jellyfin-",
	DisplayName: "Jellyfin",
	Short:       "jellyfin management",
	Long:        "jellyfin management",
	Example:     "  dashbrr service jellyfin\n  dashbrr service jellyfin --help",
	AddExample:  "  dashbrr service jellyfin add http://localhost:8096 <API-KEY>\n  dashbrr service jellyfin add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Jellyfin", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return jellyfin.NewJellyfinService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceJellyfinCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "jellyfin",
		Short:        jellyfinServiceSpec.Short,
		Long:         jellyfinServiceSpec.Long,
		Example:      jellyfinServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceJellyfinListCommand())
	command.AddCommand(ServiceJellyfinAddCommand())
	command.AddCommand(ServiceJellyfinRemoveCommand())

	return command
}

func ServiceJellyfinListCommand() *cobra.Command { return newServiceListCommand(jellyfinServiceSpec) }
func ServiceJellyfinAddCommand() *cobra.Command  { return newServiceAddCommand(jellyfinServiceSpec) }
func ServiceJellyfinRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(jellyfinServiceSpec)
}
