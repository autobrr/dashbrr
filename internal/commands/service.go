// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"

	"github.com/rs/zerolog/log"
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

func initializeCache() (cache.Store, error) {
	// Initialize cache with database directory for session storage
	cacheConfig := cache.Config{
		DataDir: filepath.Dir(os.Getenv("DASHBRR__DB_PATH")), // Use same directory as database
		Type:    cache.CacheTypeMemory,
	}
	// Determine cache type based on environment and Redis configuration
	//log.Debug().Str("type", string(cacheConfig.Type)).Msg("Cache initialized")

	// Configure Redis if enabled
	// TODO move into config
	if os.Getenv("REDIS_HOST") != "" {
		host := os.Getenv("REDIS_HOST")
		port := os.Getenv("REDIS_PORT")
		if port == "" {
			port = "6379"
		}
		cacheConfig.RedisAddr = host + ":" + port

		if os.Getenv("CACHE_TYPE") == "redis" && os.Getenv("REDIS_HOST") != "" {
			cacheConfig.Type = cache.CacheTypeRedis
		}
	}

	store, err := cache.InitCache(context.Background(), cacheConfig)
	if err != nil {
		// This should never happen as InitCache always returns a valid store
		log.Error().Err(err).Msg("Failed to initialize cache")
		return nil, err
	}
	return store, nil
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

	//command.AddCommand(ServiceListCommand())

	command.AddCommand(ServiceAutobrrCommand())
	command.AddCommand(ServiceGeneralCommand())
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

//func ServiceListCommand() *cobra.Command {
//	command := &cobra.Command{
//		Use:   "list",
//		Short: "list",
//		Long:  `list`,
//		Example: `  dashbrr service list
//  dashbrr service list --help`,
//		//SilenceUsage: true,
//	}
//
//	command.RunE = func(cmd *cobra.Command, args []string) error {
//		//"Manage service configurations",
//		//	"<service-type> <action> [arguments]\n\n"+
//		//		"  Service Types:\n"+
//		//		"    autobrr    - Autobrr service management\n"+
//		//		"    maintainerr - Maintainerr service management\n"+
//		//		"    omegabrr   - Omegabrr service management\n\n"+
//		//		"    overseerr  - Overseerr service management\n"+
//		//		"    plex       - Plex service management\n"+
//		//		"    prowlarr   - Prowlarr service management\n"+
//		//		"    radarr     - Radarr service management\n"+
//		//		"    sonarr     - Sonarr service management\n"+
//		//		"    tailscale  - Tailscale service management\n"+
//		//		"    general    - General service management\n"+
//		//		"  Use 'dashbrr run help service <service-type>' for more information",
//		return cmd.Usage()
//	}
//
//	command.AddCommand(ServiceAutobrrListCommand())
//
//	return command
//}
