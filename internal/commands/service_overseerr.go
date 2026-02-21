package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/spf13/cobra"
)

var overseerrServiceSpec = serviceSpec{
	Use:         "overseerr",
	Prefix:      "overseerr-",
	DisplayName: "Overseerr",
	Short:       "overseerr management",
	Long:        "overseerr management",
	Example:     "  dashbrr service overseerr\n  dashbrr service overseerr --help",
	AddExample:  "  dashbrr service overseerr add http://localhost:5055 <API-KEY>\n  dashbrr service overseerr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Overseerr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return overseerr.NewOverseerrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceOverseerrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "overseerr",
		Short:        overseerrServiceSpec.Short,
		Long:         overseerrServiceSpec.Long,
		Example:      overseerrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceOverseerrListCommand())
	command.AddCommand(ServiceOverseerrAddCommand())
	command.AddCommand(ServiceOverseerrRemoveCommand())

	return command
}

func ServiceOverseerrListCommand() *cobra.Command { return newServiceListCommand(overseerrServiceSpec) }
func ServiceOverseerrAddCommand() *cobra.Command  { return newServiceAddCommand(overseerrServiceSpec) }
func ServiceOverseerrRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(overseerrServiceSpec)
}
