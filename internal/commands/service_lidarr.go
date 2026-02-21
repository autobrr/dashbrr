package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/lidarr"
	"github.com/spf13/cobra"
)

var lidarrServiceSpec = serviceSpec{
	Use:         "lidarr",
	Prefix:      "lidarr-",
	DisplayName: "Lidarr",
	Short:       "lidarr management",
	Long:        "lidarr management",
	Example:     "  dashbrr service lidarr\n  dashbrr service lidarr --help",
	AddExample:  "  dashbrr service lidarr add http://localhost:8686 <API-KEY>\n  dashbrr service lidarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Lidarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return lidarr.NewLidarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceLidarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "lidarr",
		Short:        lidarrServiceSpec.Short,
		Long:         lidarrServiceSpec.Long,
		Example:      lidarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceLidarrListCommand())
	command.AddCommand(ServiceLidarrAddCommand())
	command.AddCommand(ServiceLidarrRemoveCommand())

	return command
}

func ServiceLidarrListCommand() *cobra.Command   { return newServiceListCommand(lidarrServiceSpec) }
func ServiceLidarrAddCommand() *cobra.Command    { return newServiceAddCommand(lidarrServiceSpec) }
func ServiceLidarrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(lidarrServiceSpec) }
