package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"

	"github.com/spf13/cobra"
)

// ConfigFromFlags resolves the config file, database path, and listen address from
// the flags of cmd and the environment. Serve and every CLI command use it.
func ConfigFromFlags(cmd *cobra.Command) (*config.Config, string, error) {
	return config.Load(config.Flags{
		ConfigPath: changedFlag(cmd, "config"),
		DBPath:     changedFlag(cmd, "db-file"),
		ListenAddr: changedFlag(cmd, "listen-addr"),
	})
}

// changedFlag returns the flag value only when the user set it on the command line.
func changedFlag(cmd *cobra.Command, name string) string {
	if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
		return f.Value.String()
	}
	return ""
}

// InitCache starts the session cache in the directory of the database, so
// serve and the CLI commands use the same cache files.
func InitCache(ctx context.Context, dbPath string) cache.Store {
	// cache.InitCache never returns an error.
	store, _ := cache.InitCache(ctx, cache.Config{DataDir: filepath.Dir(dbPath)})
	return store
}

// initializeDatabase opens the database and the cache that serve uses with the
// same flags and environment.
func initializeDatabase(cmd *cobra.Command) (*database.DB, error) {
	cfg, _, err := ConfigFromFlags(cmd)
	if err != nil {
		return nil, err
	}
	db, err := database.InitDBWithConfig(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}
	InitCache(cmd.Context(), cfg.Database.Path)
	return db, nil
}

func ServiceCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "service",
		Short: "Manage services",
		Long:  `Manage services`,
		Example: `  dashbrr service 
  dashbrr service --help`,
		//SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServiceListCommand())

	command.AddCommand(ServiceAutobrrCommand())
	command.AddCommand(ServiceBazarrCommand())
	command.AddCommand(ServiceGeneralCommand())
	command.AddCommand(ServiceJellyfinCommand())
	command.AddCommand(ServiceUptimeKumaCommand())
	command.AddCommand(ServiceLidarrCommand())
	command.AddCommand(ServiceMaintainerrCommand())
	command.AddCommand(ServiceOverseerrCommand())
	command.AddCommand(ServicePlexCommand())
	command.AddCommand(ServiceProwlarrCommand())
	command.AddCommand(ServiceQuiCommand())
	command.AddCommand(ServiceRadarrCommand())
	command.AddCommand(ServiceReadarrCommand())
	command.AddCommand(ServiceSabnzbdCommand())
	command.AddCommand(ServiceNzbgetCommand())
	command.AddCommand(ServiceSonarrCommand())
	command.AddCommand(ServiceTailscaleCommand())
	command.AddCommand(ServiceTraefikCommand())
	command.AddCommand(ServiceWhisparrCommand())

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
