package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/spf13/cobra"
)

var prowlarrServiceSpec = serviceSpec{
	Use:         "prowlarr",
	Prefix:      "prowlarr-",
	DisplayName: "Prowlarr",
	Short:       "prowlarr management",
	Long:        "prowlarr management",
	Example:     "  dashbrr service prowlarr\n  dashbrr service prowlarr --help",
	AddExample:  "  dashbrr service prowlarr add http://localhost:9696 <API-KEY>\n  dashbrr service prowlarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Prowlarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return prowlarr.NewProwlarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceProwlarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "prowlarr",
		Short:        prowlarrServiceSpec.Short,
		Long:         prowlarrServiceSpec.Long,
		Example:      prowlarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceProwlarrListCommand())
	command.AddCommand(ServiceProwlarrAddCommand())
	command.AddCommand(ServiceProwlarrRemoveCommand())

	return command
}

func ServiceProwlarrListCommand() *cobra.Command { return newServiceListCommand(prowlarrServiceSpec) }
func ServiceProwlarrAddCommand() *cobra.Command  { return newServiceAddCommand(prowlarrServiceSpec) }
func ServiceProwlarrRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(prowlarrServiceSpec)
}
