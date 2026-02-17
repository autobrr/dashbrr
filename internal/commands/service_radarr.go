package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/spf13/cobra"
)

var radarrServiceSpec = serviceSpec{
	Use:         "radarr",
	Prefix:      "radarr-",
	DisplayName: "Radarr",
	Short:       "radarr management",
	Long:        "radarr management",
	Example:     "  dashbrr service radarr\n  dashbrr service radarr --help",
	AddExample:  "  dashbrr service radarr add http://localhost:7878 <API-KEY>\n  dashbrr service radarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Radarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return radarr.NewRadarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceRadarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "radarr",
		Short:        radarrServiceSpec.Short,
		Long:         radarrServiceSpec.Long,
		Example:      radarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceRadarrListCommand())
	command.AddCommand(ServiceRadarrAddCommand())
	command.AddCommand(ServiceRadarrRemoveCommand())

	return command
}

func ServiceRadarrListCommand() *cobra.Command   { return newServiceListCommand(radarrServiceSpec) }
func ServiceRadarrAddCommand() *cobra.Command    { return newServiceAddCommand(radarrServiceSpec) }
func ServiceRadarrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(radarrServiceSpec) }
