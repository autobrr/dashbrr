// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/api/routes"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/logger"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/web"
)

// Build information. Populated at build-time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	logger.Init()
}

func main() {
	// Log version info at startup
	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("build_date", date).
		Msg("Starting dashbrr")

	// Parse command line flags (lowest priority)
	configPath := flag.String("config", "config.toml", "path to config file")
	dbPath := flag.String("db", "./data/dashbrr.db", "path to database file")
	listenAddr := flag.String("listen", ":8080", "address to listen on")
	flag.Parse()

	// Load configuration from file (medium priority)
	// Environment variables are automatically loaded and override file values
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		// If config file doesn't exist or has errors, create a default config
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

	// Initialize database
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// Initialize health service
	healthService := services.NewHealthService()

	// Set Gin mode based on environment
	if os.Getenv("GIN_MODE") != "release" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Gin router
	r := gin.New()

	// Use zerolog middleware and recovery
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	// In development mode, trust all proxies to allow IP access
	if gin.Mode() == gin.DebugMode {
		err = r.SetTrustedProxies(nil) // Trust all proxies in development
	} else {
		// In production, only trust loopback addresses
		err = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to set trusted proxies")
	}

	// Configure CORS
	r.Use(middleware.SetupCORS())

	// Setup API routes with database and health service
	redisCache := routes.SetupRoutes(r, db, healthService)
	defer func() {
		if err := redisCache.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close Redis cache")
		}
	}()

	// Setup static file serving
	web.ServeStatic(r)

	// Create HTTP server with proper timeouts
	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
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

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}
