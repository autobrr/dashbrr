// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"embed"
	"flag"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/api/middleware"
	"github.com/autobrr/dashbrr/backend/api/routes"
	"github.com/autobrr/dashbrr/backend/database"
	"github.com/autobrr/dashbrr/backend/services"
)

//go:embed web/dist/*
var embeddedFiles embed.FS

func init() {
	colors := map[string]string{
		"trace": "\033[36m", // Cyan
		"debug": "\033[33m", // Yellow
		"info":  "\033[34m", // Blue
		"warn":  "\033[33m", // Yellow
		"error": "\033[31m", // Red
		"fatal": "\033[35m", // Magenta
		"panic": "\033[35m", // Magenta
	}

	output := zerolog.ConsoleWriter{
		Out:     os.Stdout,
		NoColor: false,
		FormatLevel: func(i interface{}) string {
			level, ok := i.(string)
			if !ok {
				return "???"
			}
			color := colors[level]
			if color == "" {
				color = "\033[37m" // Default to white
			}
			return color + strings.ToUpper(level) + "\033[0m"
		},
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

func main() {
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

	// Get the embedded files sub-directory
	dist, err := fs.Sub(embeddedFiles, "web/dist")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get embedded files")
	}

	// Serve static files with proper MIME types and headers
	r.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := strings.TrimPrefix(c.Param("filepath"), "/")

		// Open the file from embedded filesystem
		file, err := dist.Open(path.Join("assets", filepath))
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		// Read file info to get content type
		stat, err := file.Stat()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		// Set content type based on file extension
		ext := strings.ToLower(path.Ext(filepath))
		var contentType string
		switch ext {
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "application/javascript; charset=utf-8"
		case ".svg":
			contentType = "image/svg+xml"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		default:
			contentType = "application/octet-stream"
		}

		// Set headers
		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=31536000")
		c.Header("X-Content-Type-Options", "nosniff")

		// Stream the file
		c.DataFromReader(http.StatusOK, stat.Size(), contentType, file, nil)
	})

	// Serve specific static files
	r.GET("/favicon.ico", func(c *gin.Context) {
		file, err := dist.Open("favicon.ico")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		c.Header("Content-Type", "image/x-icon")
		c.Header("Cache-Control", "public, max-age=31536000")
		io.Copy(c.Writer, file)
	})

	// Serve index.html for root path with proper headers
	r.GET("/", serveIndex(dist))

	// Handle all other routes
	r.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.AbortWithStatus(404)
			return
		}

		// For all other routes, serve index.html for client-side routing
		serveIndex(dist)(c)
	})

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

// serveIndex is a helper function to serve index.html with proper headers
func serveIndex(dist fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := dist.Open("index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		io.Copy(c.Writer, file)
	}
}
