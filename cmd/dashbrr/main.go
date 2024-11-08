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

	// Parse command line flags
	dbPath := flag.String("db", "./data/dashbrr.db", "path to database file")
	listenAddr := flag.String("listen", ":8080", "address to listen on")
	flag.Parse()

	// Use environment variables if set, otherwise use flag values
	finalDbPath := os.Getenv("DASHBRR__DB_PATH")
	if finalDbPath == "" {
		finalDbPath = *dbPath
	}

	finalListenAddr := os.Getenv("DASHBRR__LISTEN_ADDR")
	if finalListenAddr == "" {
		finalListenAddr = *listenAddr
	}

	// Initialize database
	db, err := database.InitDB(finalDbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// Initialize health service
	healthService := services.NewHealthService()

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Use zerolog middleware and recovery
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	// Configure trusted proxies - only trust loopback addresses
	err = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
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
		Addr:         finalListenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info().Msgf("Starting server on %s", finalListenAddr)
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
