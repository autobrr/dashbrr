package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/spf13/cobra"
)

var maintainerrServiceSpec = serviceSpec{
	Use:         "maintainerr",
	Prefix:      "maintainerr-",
	DisplayName: "Maintainerr",
	Short:       "maintainerr management",
	Long:        "maintainerr management",
	Example:     "  dashbrr service maintainerr\n  dashbrr service maintainerr --help",
	AddExample:  "  dashbrr service maintainerr add http://localhost:6246 <API-KEY>\n  dashbrr service maintainerr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Maintainerr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return maintainerr.NewMaintainerrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceMaintainerrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "maintainerr",
		Short:        maintainerrServiceSpec.Short,
		Long:         maintainerrServiceSpec.Long,
		Example:      maintainerrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceMaintainerrListCommand())
	command.AddCommand(ServiceMaintainerrAddCommand())
	command.AddCommand(ServiceMaintainerrRemoveCommand())

	return command
}

func ServiceMaintainerrListCommand() *cobra.Command {
	return newServiceListCommand(maintainerrServiceSpec)
}
func ServiceMaintainerrAddCommand() *cobra.Command {
	return newServiceAddCommand(maintainerrServiceSpec)
}
func ServiceMaintainerrRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(maintainerrServiceSpec)
}
