package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/bazarr"
	"github.com/spf13/cobra"
)

var bazarrServiceSpec = serviceSpec{
	Use:         "bazarr",
	Prefix:      "bazarr-",
	DisplayName: "Bazarr",
	Short:       "bazarr management",
	Long:        "bazarr management",
	Example:     "  dashbrr service bazarr\n  dashbrr service bazarr --help",
	AddExample:  "  dashbrr service bazarr add http://localhost:6767 <API-KEY>\n  dashbrr service bazarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Bazarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return bazarr.NewBazarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceBazarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "bazarr",
		Short:        bazarrServiceSpec.Short,
		Long:         bazarrServiceSpec.Long,
		Example:      bazarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceBazarrListCommand())
	command.AddCommand(ServiceBazarrAddCommand())
	command.AddCommand(ServiceBazarrRemoveCommand())

	return command
}

func ServiceBazarrListCommand() *cobra.Command   { return newServiceListCommand(bazarrServiceSpec) }
func ServiceBazarrAddCommand() *cobra.Command    { return newServiceAddCommand(bazarrServiceSpec) }
func ServiceBazarrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(bazarrServiceSpec) }
