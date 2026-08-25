package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/whisparr"
)

var whisparrServiceSpec = serviceSpec{
	Use:         "whisparr",
	Prefix:      "whisparr-",
	DisplayName: "Whisparr",
	Short:       "whisparr management",
	Long:        "whisparr management",
	Example:     "  dashbrr service whisparr\n  dashbrr service whisparr --help",
	AddExample:  "  dashbrr service whisparr add http://localhost:6969 <API-KEY>\n  dashbrr service whisparr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Whisparr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return whisparr.NewWhisparrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceWhisparrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "whisparr",
		Short:        whisparrServiceSpec.Short,
		Long:         whisparrServiceSpec.Long,
		Example:      whisparrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceWhisparrListCommand())
	command.AddCommand(ServiceWhisparrAddCommand())
	command.AddCommand(ServiceWhisparrRemoveCommand())

	return command
}

func ServiceWhisparrListCommand() *cobra.Command { return newServiceListCommand(whisparrServiceSpec) }
func ServiceWhisparrAddCommand() *cobra.Command  { return newServiceAddCommand(whisparrServiceSpec) }
func ServiceWhisparrRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(whisparrServiceSpec)
}
