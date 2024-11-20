// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/autobrr/dashbrr/internal/api"
	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/commands"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
	"github.com/autobrr/dashbrr/internal/services"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	logger.Init()
}

func main() {
	var rootCmd = &cobra.Command{
		Use: "dashbrr",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.AddCommand(commands.ConfigCommand())
	rootCmd.AddCommand(commands.ServiceCommand())
	rootCmd.AddCommand(commands.VersionCommand())
	rootCmd.AddCommand(commands.UserCommand())
	rootCmd.AddCommand(commands.HealthCommand())

	rootCmd.AddCommand(ServeCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func ServeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run dashbrr service",
		Long:  `serve runs dashbrr`,
		Example: `  dashbrr serve
  dashbrr serve --help`,
		//SilenceUsage: true,
	}

	var (
		configPath = "config.toml"
		listenAddr = ":8080"
		dbFile     = ""
	)

	command.Flags().StringVar(&configPath, "config", "config.toml", "path to config file")
	command.Flags().StringVar(&listenAddr, "listen-addr", listenAddr, "address to listen on")
	command.Flags().StringVar(&dbFile, "db-file", "", "path to database file")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return startServer(configPath, listenAddr, dbFile)
	}

	return command
}

func startServer(configPath string, listenAddr string, origDBPath string) error {
	log.Info().
		Str("version", buildinfo.Version).
		Str("commit", buildinfo.Commit).
		Str("build_date", buildinfo.Date).
		Msg("Starting dashbrr")

	// Check environment variable first, then fall back to flag
	defaultConfigPath := "config.toml"
	if envPath := os.Getenv(config.EnvConfigPath); envPath != "" {
		defaultConfigPath = envPath
	} else {
		// Check user config directory
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			log.Error().Err(err).Msg("failed to get user config directory")
		}

		base := []string{filepath.Join(userConfigDir, "dashbrr"), "/config"}
		configs := []string{"config.toml", "config.yaml", "config.yml"}

		for _, b := range base {
			for _, c := range configs {
				p := filepath.Join(b, c)
				if _, err := os.Stat(p); err == nil {
					defaultConfigPath = p
					break
				}
			}
			if defaultConfigPath != "config.toml" {
				break
			}
		}
	}

	// Store original flag values to detect changes
	origListenAddr := ":8080"

	// If dbPath wasn't set via flag, use config directory
	if origDBPath == "" {
		configDir := filepath.Dir(configPath)
		origDBPath = filepath.Join(configDir, "data", "dashbrr.db")
	}

	var cfg *config.Config
	var err error

	if config.HasRequiredEnvVars() {
		cfg = &config.Config{}
		if err := config.LoadEnvOverrides(cfg); err != nil {
			log.Error().Err(err).Msg("Failed to load environment variables")
			return err
		}
	} else {
		log.Debug().Str("path", configPath).Msg("Loading config file")

		cfg, err = config.LoadConfig(configPath)
		if err != nil {
			log.Error().Err(err).Msg("Failed to load or create configuration")
			return err
		}

		// Override with command line flags if they differ from defaults
		if listenAddr != origListenAddr {
			cfg.Server.ListenAddr = listenAddr
		}
		if flag.Lookup("db") != nil && origDBPath != "" {
			cfg.Database.Path = origDBPath
		}
	}

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize database")
		return err
	}
	defer db.Close()

	// Create a root context for cache initialization
	ctx := context.Background()

	// Initialize cache with database directory for session storage
	cacheConfig := cache.Config{
		DataDir: filepath.Dir(os.Getenv("DASHBRR__DB_PATH")), // Use same directory as database
		Type:    cache.CacheTypeMemory,
	}
	// Determine cache type based on environment and Redis configuration
	log.Debug().Str("type", string(cacheConfig.Type)).Msg("Cache initialized")

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

	store, err := cache.InitCache(ctx, cacheConfig)
	if err != nil {
		// This should never happen as InitCache always returns a valid store
		log.Error().Err(err).Msg("Failed to initialize cache")
		return err
	}

	serviceManager := services.NewServiceManager(db, store)
	if err := serviceManager.InitializeServices(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to initialize services")
	}

	serviceManager.StartHealthMonitor()

	// TODO remove
	healthService := services.NewHealthService()

	srv := api.NewServer(cfg, db, store, serviceManager, healthService)

	errorChannel := make(chan error)
	go func() {
		listenErr := srv.ListenAndServe()
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errorChannel <- listenErr
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info().Msgf("got signal %v, shutting down server", sig.String())
	case err := <-errorChannel:
		log.Error().Err(err).Msg("got unexpected error from server")
	}

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("got error during graceful http shutdown")

		os.Exit(1)
	}

	if err := store.Close(); err != nil {
		log.Error().Err(err).Msg("failed to close cache connection")
	}

	os.Exit(0)

	return nil
}
