package commands

import (
	"context"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/readarr"
	"github.com/spf13/cobra"
)

var readarrServiceSpec = serviceSpec{
	Use:         "readarr",
	Prefix:      "readarr-",
	DisplayName: "Readarr",
	Short:       "readarr management",
	Long:        "readarr management",
	Example:     "  dashbrr service readarr\n  dashbrr service readarr --help",
	AddExample:  "  dashbrr service readarr add http://localhost:8787 <API-KEY>\n  dashbrr service readarr add --help",
	AddArgs:     cobra.MinimumNArgs(2),
	ParseAdd: func(args []string) (serviceURL, apiKey, displayName string, err error) {
		return args[0], args[1], "Readarr", nil
	},
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return readarr.NewReadarrService().CheckHealth(ctx, serviceURL, apiKey)
	},
}

func ServiceReadarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "readarr",
		Short:        readarrServiceSpec.Short,
		Long:         readarrServiceSpec.Long,
		Example:      readarrServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceReadarrListCommand())
	command.AddCommand(ServiceReadarrAddCommand())
	command.AddCommand(ServiceReadarrRemoveCommand())

	return command
}

func ServiceReadarrListCommand() *cobra.Command   { return newServiceListCommand(readarrServiceSpec) }
func ServiceReadarrAddCommand() *cobra.Command    { return newServiceAddCommand(readarrServiceSpec) }
func ServiceReadarrRemoveCommand() *cobra.Command { return newServiceRemoveCommand(readarrServiceSpec) }
