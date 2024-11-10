// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/api/routes"
	"github.com/autobrr/dashbrr/internal/commands"
	"github.com/autobrr/dashbrr/internal/commands/autobrr"
	"github.com/autobrr/dashbrr/internal/commands/health"
	"github.com/autobrr/dashbrr/internal/commands/help"
	"github.com/autobrr/dashbrr/internal/commands/omegabrr"
	"github.com/autobrr/dashbrr/internal/commands/service"
	"github.com/autobrr/dashbrr/internal/commands/user"
	"github.com/autobrr/dashbrr/internal/commands/version"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/web"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	logger.Init()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		if err := executeCommand(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	startServer()
}

func executeCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("no command specified\n\nRun 'dashbrr run help' for usage")
	}

	// Initialize database for user commands
	dbPath := "./data/dashbrr.db"
	db, err := database.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	registry := commands.NewRegistry()
	helpCmd := help.NewHelpCommand(registry)
	serviceCmd := service.NewServiceCommand()

	// Register top-level commands
	registry.Register(version.NewVersionCommand())
	registry.Register(health.NewHealthCommand())
	registry.Register(helpCmd)
	registry.Register(user.NewUserCommand(db))
	registry.Register(serviceCmd)

	// Register service-specific commands
	registry.Register(autobrr.NewAddCommand(db))
	registry.Register(autobrr.NewRemoveCommand(db))
	registry.Register(autobrr.NewListCommand(db))

	// Register omegabrr commands
	registry.Register(omegabrr.NewAddCommand(db))
	registry.Register(omegabrr.NewRemoveCommand(db))
	registry.Register(omegabrr.NewListCommand(db))

	// Set registry on commands that need it
	serviceCmd.SetRegistry(registry)

	// Extract command name and arguments
	args := os.Args[2:]
	cmdName := strings.Join(args[:len(args)-len(args[1:])], " ")
	cmdArgs := args[1:]

	return registry.Execute(context.Background(), cmdName, cmdArgs)
}

func startServer() {
	log.Info().
		Str("version", Version).
		Str("commit", Commit).
		Str("build_date", Date).
		Msg("Starting dashbrr")

	configPath := flag.String("config", "config.toml", "path to config file")
	dbPath := flag.String("db", "./data/dashbrr.db", "path to database file")
	listenAddr := flag.String("listen", ":8080", "address to listen on")
	flag.Parse()

	var cfg *config.Config
	var err error

	if config.HasRequiredEnvVars() {
		cfg = &config.Config{}
		if err := config.LoadEnvOverrides(cfg); err != nil {
			log.Fatal().Err(err).Msg("Failed to load environment variables")
		}
	} else {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			cfg = &config.Config{
				Server: config.ServerConfig{
					ListenAddr: *listenAddr,
				},
				Database: config.DatabaseConfig{
					Path: *dbPath,
				},
			}
			log.Warn().Err(err).Msg("Failed to load configuration file, using defaults")
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
