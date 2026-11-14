package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/netbird"
)

var netbirdServiceSpec = serviceSpec{
	Use:         "netbird",
	Prefix:      "netbird-",
	DisplayName: "NetBird",
	Short:       "netbird management",
	Long:        "netbird management",
	Example:     "  dashbrr service netbird\n  dashbrr service netbird --help",
	AddExample:  "  dashbrr service netbird add https://api.netbird.io <API-TOKEN>\n  dashbrr service netbird add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "NetBird", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return netbird.NewNetBirdService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceNetBirdCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "netbird",
		Short:        netbirdServiceSpec.Short,
		Long:         netbirdServiceSpec.Long,
		Example:      netbirdServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceNetBirdListCommand())
	command.AddCommand(ServiceNetBirdAddCommand())
	command.AddCommand(ServiceNetBirdRemoveCommand())

	return command
}

func ServiceNetBirdListCommand() *cobra.Command { return newServiceListCommand(netbirdServiceSpec) }
func ServiceNetBirdAddCommand() *cobra.Command  { return newServiceAddCommand(netbirdServiceSpec) }
func ServiceNetBirdRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(netbirdServiceSpec)
}
