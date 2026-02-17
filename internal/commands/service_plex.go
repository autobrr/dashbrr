package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/spf13/cobra"
)

var plexServiceSpec = serviceSpec{
	Use:         "plex",
	Prefix:      "plex-",
	DisplayName: "Plex",
	Short:       "plex management",
	Long:        "plex management",
	Example:     "  dashbrr service plex\n  dashbrr service plex --help",
	AddExample:  "  dashbrr service plex add http://localhost:32400 <TOKEN>\n  dashbrr service plex add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Plex", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return plex.NewPlexService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServicePlexCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "plex",
		Short:        plexServiceSpec.Short,
		Long:         plexServiceSpec.Long,
		Example:      plexServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServicePlexListCommand())
	command.AddCommand(ServicePlexAddCommand())
	command.AddCommand(ServicePlexRemoveCommand())

	return command
}

func ServicePlexListCommand() *cobra.Command   { return newServiceListCommand(plexServiceSpec) }
func ServicePlexAddCommand() *cobra.Command    { return newServiceAddCommand(plexServiceSpec) }
func ServicePlexRemoveCommand() *cobra.Command { return newServiceRemoveCommand(plexServiceSpec) }
