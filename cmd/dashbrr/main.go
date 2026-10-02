// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/autobrr/dashbrr/internal/api"
	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/commands"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
	"github.com/autobrr/dashbrr/internal/services/cache"

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

	rootCmd.PersistentFlags().String("config", "", "path to config file")
	rootCmd.PersistentFlags().String("db-file", "", "path to database file")

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

	command.Flags().String("listen-addr", ":8080", "address to listen on")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return startServer(cmd)
	}

	return command
}

func startServer(cmd *cobra.Command) error {
	log.Info().
		Str("version", buildinfo.Version).
		Str("commit", buildinfo.Commit).
		Str("build_date", buildinfo.Date).
		Msg("Starting dashbrr")

	cfg, configPath, err := commands.ConfigFromFlags(cmd)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load or create configuration")
		return err
	}
	log.Debug().Str("path", configPath).Str("database", cfg.Database.Path).Msg("Loaded config")

	db, err := database.InitDBWithConfig(&cfg.Database)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize database")
		return err
	}
	defer db.Close()

	// Create a root context for cache initialization
	ctx := context.Background()

	// Initialize cache with database directory for session storage
	cacheConfig := cache.Config{
		DataDir: filepath.Dir(cfg.Database.Path),
	}
	if cacheConfig.DataDir == "." || cacheConfig.DataDir == "" {
		cacheConfig.DataDir = "./data"
	}
	log.Debug().Msg("Cache initialized")

	store, err := cache.InitCache(ctx, cacheConfig)
	if err != nil {
		// This should never happen as InitCache always returns a valid store
		log.Error().Err(err).Msg("Failed to initialize cache")
		return err
	}

	srv := api.NewServer(cfg, db, store)

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
