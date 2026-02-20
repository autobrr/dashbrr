package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/nzbget"
	"github.com/spf13/cobra"
)

var nzbgetServiceSpec = serviceSpec{
	Use:         "nzbget",
	Prefix:      "nzbget-",
	DisplayName: "NZBGet",
	Short:       "nzbget management",
	Long:        "nzbget management",
	Example:     "  dashbrr service nzbget\n  dashbrr service nzbget --help",
	AddExample:  "  dashbrr service nzbget add http://localhost:6789 <CONTROL-PASSWORD>\n  dashbrr service nzbget add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "NZBGet", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return nzbget.NewNzbgetService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceNzbgetCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "nzbget",
		Short:        nzbgetServiceSpec.Short,
		Long:         nzbgetServiceSpec.Long,
		Example:      nzbgetServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceNzbgetListCommand())
	command.AddCommand(ServiceNzbgetAddCommand())
	command.AddCommand(ServiceNzbgetRemoveCommand())

	return command
}

func ServiceNzbgetListCommand() *cobra.Command   { return newServiceListCommand(nzbgetServiceSpec) }
func ServiceNzbgetAddCommand() *cobra.Command    { return newServiceAddCommand(nzbgetServiceSpec) }
func ServiceNzbgetRemoveCommand() *cobra.Command { return newServiceRemoveCommand(nzbgetServiceSpec) }
