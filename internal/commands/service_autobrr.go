package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/spf13/cobra"
)

var autobrrServiceSpec = serviceSpec{
	Use:         "autobrr",
	Prefix:      "autobrr-",
	DisplayName: "Autobrr",
	Short:       "autobrr management",
	Long:        "autobrr torrents",
	Example:     "  dashbrr service autobrr\n  dashbrr service autobrr --help",
	AddExample:  "  dashbrr service autobrr add http://localhost:7474 <API-KEY>\n  dashbrr service autobrr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Autobrr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return autobrr.NewAutobrrService().CheckHealth(ctx, serviceURL, apiKey)
	},
	HealthOK: func(health models.ServiceHealth) bool {
		return health.Status == "online"
	},
}

func ServiceAutobrrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "autobrr",
		Short:        autobrrServiceSpec.Short,
		Long:         autobrrServiceSpec.Long,
		Example:      autobrrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceAutobrrListCommand())
	command.AddCommand(ServiceAutobrrAddCommand())
	command.AddCommand(ServiceAutobrrRemoveCommand())

	return command
}

func ServiceAutobrrListCommand() *cobra.Command   { return newServiceListCommand(autobrrServiceSpec) }
func ServiceAutobrrAddCommand() *cobra.Command    { return newServiceAddCommand(autobrrServiceSpec) }
func ServiceAutobrrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(autobrrServiceSpec) }
