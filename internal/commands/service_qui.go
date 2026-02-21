package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/qui"
	"github.com/spf13/cobra"
)

var quiServiceSpec = serviceSpec{
	Use:         "qui",
	Prefix:      "qui-",
	DisplayName: "Qui",
	Short:       "qui management",
	Long:        "qui management",
	Example:     "  dashbrr service qui\n  dashbrr service qui --help",
	AddExample:  "  dashbrr service qui add http://localhost:7476 <API-KEY>\n  dashbrr service qui add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Qui", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return qui.NewQuiService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceQuiCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "qui",
		Short:        quiServiceSpec.Short,
		Long:         quiServiceSpec.Long,
		Example:      quiServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceQuiListCommand())
	command.AddCommand(ServiceQuiAddCommand())
	command.AddCommand(ServiceQuiRemoveCommand())

	return command
}

func ServiceQuiListCommand() *cobra.Command   { return newServiceListCommand(quiServiceSpec) }
func ServiceQuiAddCommand() *cobra.Command    { return newServiceAddCommand(quiServiceSpec) }
func ServiceQuiRemoveCommand() *cobra.Command { return newServiceRemoveCommand(quiServiceSpec) }
