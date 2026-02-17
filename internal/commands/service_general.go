package commands

import (
	"context"
	"fmt"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/spf13/cobra"
)

var generalServiceSpec = serviceSpec{
	Use:         "generic",
	Prefix:      "general-",
	DisplayName: "General",
	Short:       "generic management",
	Long:        "generic management",
	Example:     "  dashbrr service generic\n  dashbrr service generic --help",
	AddExample: "  dashbrr service generic add [url] [name]\n" +
		"  dashbrr service generic add [url] [name] [apiKey]\n" +
		"  dashbrr service generic add --help",
	AddArgs: cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		serviceURL = args[0]
		displayName = args[1]
		apiKey = ""
		if len(args) >= 3 {
			apiKey = args[2]
		}
		if displayName == "" {
			return "", "", "", fmt.Errorf("name is required")
		}
		return serviceURL, apiKey, displayName, nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return models.NewGeneralService().CheckHealth(ctx, serviceURL, apiKey)
	},
	HealthOK: func(health models.ServiceHealth) bool {
		return health.Status == "online"
	},
}

func ServiceGeneralCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "generic",
		Short:        generalServiceSpec.Short,
		Long:         generalServiceSpec.Long,
		Example:      generalServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceGeneralListCommand())
	command.AddCommand(ServiceGeneralAddCommand())
	command.AddCommand(ServiceGeneralRemoveCommand())

	return command
}

func ServiceGeneralListCommand() *cobra.Command   { return newServiceListCommand(generalServiceSpec) }
func ServiceGeneralAddCommand() *cobra.Command    { return newServiceAddCommand(generalServiceSpec) }
func ServiceGeneralRemoveCommand() *cobra.Command { return newServiceRemoveCommand(generalServiceSpec) }
