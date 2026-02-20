package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/uptimekuma"
	"github.com/spf13/cobra"
)

var uptimeKumaServiceSpec = serviceSpec{
	Use:         "uptimekuma",
	Prefix:      "uptimekuma-",
	DisplayName: "Uptime Kuma",
	Short:       "uptimekuma management",
	Long:        "uptimekuma management",
	Example:     "  dashbrr service uptimekuma\n  dashbrr service uptimekuma --help",
	AddExample:  "  dashbrr service uptimekuma add http://localhost:3001 <API-KEY>\n  dashbrr service uptimekuma add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Uptime Kuma", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return uptimekuma.NewUptimeKumaService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceUptimeKumaCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "uptimekuma",
		Short:        uptimeKumaServiceSpec.Short,
		Long:         uptimeKumaServiceSpec.Long,
		Example:      uptimeKumaServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceUptimeKumaListCommand())
	command.AddCommand(ServiceUptimeKumaAddCommand())
	command.AddCommand(ServiceUptimeKumaRemoveCommand())

	return command
}

func ServiceUptimeKumaListCommand() *cobra.Command {
	return newServiceListCommand(uptimeKumaServiceSpec)
}
func ServiceUptimeKumaAddCommand() *cobra.Command { return newServiceAddCommand(uptimeKumaServiceSpec) }
func ServiceUptimeKumaRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(uptimeKumaServiceSpec)
}
