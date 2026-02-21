package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/spf13/cobra"
)

var tailscaleServiceSpec = serviceSpec{
	Use:         "tailscale",
	Prefix:      "tailscale-",
	DisplayName: "Tailscale",
	Short:       "tailscale management",
	Long:        "tailscale management",
	Example:     "  dashbrr service tailscale\n  dashbrr service tailscale --help",
	AddExample:  "  dashbrr service tailscale add <API-KEY>\n  dashbrr service tailscale add --help",
	AddArgs:     cobra.MinimumNArgs(1),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return "https://api.tailscale.com", args[0], "Tailscale", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return tailscale.NewTailscaleService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceTailscaleCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "tailscale",
		Short:        tailscaleServiceSpec.Short,
		Long:         tailscaleServiceSpec.Long,
		Example:      tailscaleServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceTailscaleListCommand())
	command.AddCommand(ServiceTailscaleAddCommand())
	command.AddCommand(ServiceTailscaleRemoveCommand())

	return command
}

func ServiceTailscaleListCommand() *cobra.Command { return newServiceListCommand(tailscaleServiceSpec) }
func ServiceTailscaleAddCommand() *cobra.Command  { return newServiceAddCommand(tailscaleServiceSpec) }
func ServiceTailscaleRemoveCommand() *cobra.Command {
	return newServiceRemoveCommand(tailscaleServiceSpec)
}
