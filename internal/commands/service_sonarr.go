package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/spf13/cobra"
)

var sonarrServiceSpec = serviceSpec{
	Use:         "sonarr",
	Prefix:      "sonarr-",
	DisplayName: "Sonarr",
	Short:       "sonarr management",
	Long:        "sonarr management",
	Example:     "  dashbrr service sonarr\n  dashbrr service sonarr --help",
	AddExample:  "  dashbrr service sonarr add http://localhost:8989 <API-KEY>\n  dashbrr service sonarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Sonarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return sonarr.NewSonarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceSonarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "sonarr",
		Short:        sonarrServiceSpec.Short,
		Long:         sonarrServiceSpec.Long,
		Example:      sonarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceSonarrListCommand())
	command.AddCommand(ServiceSonarrAddCommand())
	command.AddCommand(ServiceSonarrRemoveCommand())

	return command
}

func ServiceSonarrListCommand() *cobra.Command   { return newServiceListCommand(sonarrServiceSpec) }
func ServiceSonarrAddCommand() *cobra.Command    { return newServiceAddCommand(sonarrServiceSpec) }
func ServiceSonarrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(sonarrServiceSpec) }
