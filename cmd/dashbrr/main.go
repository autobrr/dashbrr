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
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/api/routes"
	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/commands"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/web"
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

	rootCmd.AddCommand(RunCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	//startServer()
}

func RunCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "run",
		Short: "run",
		Long:  `run`,
		Example: `  dashbrr run
  dashbrr run --help`,
		//SilenceUsage: true,
	}

	var (
		outputJson  = false
		checkUpdate = false
	)

	command.Flags().BoolVar(&outputJson, "json", false, "output in JSON format")
	command.Flags().BoolVar(&checkUpdate, "check-github", false, "check for updates")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		startServer()
		return nil
	}

	return command
}

func startServer() {
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
	var origDBPath string

	configPath := flag.String("config", defaultConfigPath, "path to config file")
	listenAddr := flag.String("listen", origListenAddr, "address to listen on")
	flag.StringVar(&origDBPath, "db", "", "path to database file")
	flag.Parse()

	// If dbPath wasn't set via flag, use config directory
	if origDBPath == "" {
		configDir := filepath.Dir(*configPath)
		origDBPath = filepath.Join(configDir, "data", "dashbrr.db")
	}

	var cfg *config.Config
	var err error

	if config.HasRequiredEnvVars() {
		cfg = &config.Config{}
		if err := config.LoadEnvOverrides(cfg); err != nil {
			log.Fatal().Err(err).Msg("Failed to load environment variables")
		}
	} else {
		log.Debug().Str("path", *configPath).Msg("Loading config file")

		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load or create configuration")
		}

		// Override with command line flags if they differ from defaults
		if *listenAddr != origListenAddr {
			cfg.Server.ListenAddr = *listenAddr
		}
		if flag.Lookup("db") != nil && origDBPath != "" {
			cfg.Database.Path = origDBPath
		}
	}

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	healthService := services.NewHealthService()

	if os.Getenv("GIN_MODE") == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	if gin.Mode() == gin.DebugMode {
		err = r.SetTrustedProxies(nil)
	} else {
		err = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to set trusted proxies")
	}

	r.Use(middleware.SetupCORS())

	cacheStore := routes.SetupRoutes(r, db, healthService)
	defer func() {
		if err := cacheStore.Close(); err != nil {
			cacheType := strings.ToLower(os.Getenv("CACHE_TYPE"))
			if cacheType == "redis" {
				log.Error().Err(err).Msg("Failed to close Redis cache connection")
			} else {
				log.Debug().Err(err).Msg("Cache cleanup completed")
			}
		}
	}()

	web.ServeStatic(r)

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().
			Str("address", cfg.Server.ListenAddr).
			Str("mode", gin.Mode()).
			Str("database", cfg.Database.Path).
			Msg("Starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}
