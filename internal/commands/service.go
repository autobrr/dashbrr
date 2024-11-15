package commands

import (
	"fmt"

	"github.com/autobrr/dashbrr/internal/database"

	"github.com/spf13/cobra"
)

func initializeDatabase() (*database.DB, error) {
	dbPath := "./data/dashbrr.db"
	db, err := database.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}
	return db, nil
}

func ServiceCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "service",
		Short: "service",
		Long:  `service`,
		Example: `  dashbrr service 
  dashbrr service --help`,
		//SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServiceListCommand())

	command.AddCommand(ServiceAutobrrCommand())
	command.AddCommand(ServiceMaintainerrCommand())
	command.AddCommand(ServiceOmegabrrCommand())
	command.AddCommand(ServiceOverseerrCommand())
	command.AddCommand(ServicePlexCommand())
	command.AddCommand(ServiceProwlarrCommand())
	command.AddCommand(ServiceRadarrCommand())
	command.AddCommand(ServiceSonarrCommand())
	command.AddCommand(ServiceTailscaleCommand())

	return command
}

//func ServiceAddCommand() *cobra.Command {
//	command := &cobra.Command{
//		Use:   "add",
//		Short: "add",
//		Long:  `add`,
//		Example: `  dashbrr service add
//  dashbrr service add --help`,
//		//SilenceUsage: true,
//	}
//
//	command.RunE = func(cmd *cobra.Command, args []string) error {
//		return cmd.Usage()
//	}
//
//	command.AddCommand(ServiceAutobrrAddCommand())
//
//	return command
//}

func ServiceListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "list",
		Long:  `list`,
		Example: `  dashbrr service list
  dashbrr service list --help`,
		//SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		//"Manage service configurations",
		//	"<service-type> <action> [arguments]\n\n"+
		//		"  Service Types:\n"+
		//		"    autobrr    - Autobrr service management\n"+
		//		"    maintainerr - Maintainerr service management\n"+
		//		"    omegabrr   - Omegabrr service management\n\n"+
		//		"    overseerr  - Overseerr service management\n"+
		//		"    plex       - Plex service management\n"+
		//		"    prowlarr   - Prowlarr service management\n"+
		//		"    radarr     - Radarr service management\n"+
		//		"    sonarr     - Sonarr service management\n"+
		//		"    tailscale  - Tailscale service management\n"+
		//		"    general    - General service management\n"+
		//		"  Use 'dashbrr run help service <service-type>' for more information",
		return cmd.Usage()
	}

	command.AddCommand(ServiceAutobrrListCommand())

	return command
}

//func ServiceRemoveCommand() *cobra.Command {
//	command := &cobra.Command{
//		Use:   "remove",
//		Short: "remove",
//		Long:  `remove`,
//		Example: `  dashbrr service remove
//  dashbrr service remove --help`,
//		//SilenceUsage: true,
//	}
//
//	command.RunE = func(cmd *cobra.Command, args []string) error {
//		return cmd.Usage()
//	}
//
//	command.AddCommand(ServiceAutobrrRemoveCommand())
//
//	return command
//}
