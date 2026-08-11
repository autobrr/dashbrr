package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/seerr"
	"github.com/spf13/cobra"
)

var seerrServiceSpec = serviceSpec{
	Use:         "seerr",
	Prefix:      "seerr-",
	DisplayName: "Seerr",
	Short:       "seerr management",
	Long:        "seerr management",
	Example:     "  dashbrr service seerr\n  dashbrr service seerr --help",
	AddExample:  "  dashbrr service seerr add http://localhost:5055 <API-KEY>\n  dashbrr service seerr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Seerr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return seerr.NewSeerrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceSeerrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "seerr",
		Short:        seerrServiceSpec.Short,
		Long:         seerrServiceSpec.Long,
		Example:      seerrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceSeerrListCommand())
	command.AddCommand(ServiceSeerrAddCommand())
	command.AddCommand(ServiceSeerrRemoveCommand())

	return command
}

func ServiceSeerrListCommand() *cobra.Command { return newServiceListCommand(seerrServiceSpec) }
func ServiceSeerrAddCommand() *cobra.Command  { return newServiceAddCommand(seerrServiceSpec) }
func ServiceSeerrRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(seerrServiceSpec)
}
