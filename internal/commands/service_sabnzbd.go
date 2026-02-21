package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/sabnzbd"
	"github.com/spf13/cobra"
)

var sabnzbdServiceSpec = serviceSpec{
	Use:         "sabnzbd",
	Prefix:      "sabnzbd-",
	DisplayName: "SABnzbd",
	Short:       "sabnzbd management",
	Long:        "sabnzbd management",
	Example:     "  dashbrr service sabnzbd\n  dashbrr service sabnzbd --help",
	AddExample:  "  dashbrr service sabnzbd add http://localhost:8080 <API-KEY>\n  dashbrr service sabnzbd add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "SABnzbd", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return sabnzbd.NewSabnzbdService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceSabnzbdCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "sabnzbd",
		Short:        sabnzbdServiceSpec.Short,
		Long:         sabnzbdServiceSpec.Long,
		Example:      sabnzbdServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceSabnzbdListCommand())
	command.AddCommand(ServiceSabnzbdAddCommand())
	command.AddCommand(ServiceSabnzbdRemoveCommand())

	return command
}

func ServiceSabnzbdListCommand() *cobra.Command   { return newServiceListCommand(sabnzbdServiceSpec) }
func ServiceSabnzbdAddCommand() *cobra.Command    { return newServiceAddCommand(sabnzbdServiceSpec) }
func ServiceSabnzbdRemoveCommand() *cobra.Command { return newServiceRemoveCommand(sabnzbdServiceSpec) }
