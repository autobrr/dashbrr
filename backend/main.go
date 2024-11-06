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

	// Helper function to serve static files with proper headers
	serveStaticFile := func(c *gin.Context, filepath string, contentType string) {
		file, err := dist.Open(filepath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Header("Content-Type", contentType)
		if strings.Contains(filepath, "sw.js") || strings.Contains(filepath, "manifest.json") {
			c.Header("Cache-Control", "no-cache")
			if strings.Contains(filepath, "sw.js") {
				c.Header("Service-Worker-Allowed", "/")
			}
		} else {
			c.Header("Cache-Control", "public, max-age=31536000")
		}
		c.Header("X-Content-Type-Options", "nosniff")

		c.DataFromReader(http.StatusOK, stat.Size(), contentType, file, nil)
	}

	// Serve static files from root path
	r.GET("/logo.svg", func(c *gin.Context) {
		serveStaticFile(c, "logo.svg", "image/svg+xml")
	})

	r.GET("/masked-icon.svg", func(c *gin.Context) {
		serveStaticFile(c, "masked-icon.svg", "image/svg+xml")
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		serveStaticFile(c, "favicon.ico", "image/x-icon")
	})

	r.GET("/apple-touch-icon.png", func(c *gin.Context) {
		serveStaticFile(c, "apple-touch-icon.png", "image/png")
	})

	r.GET("/apple-touch-icon-iphone-60x60.png", func(c *gin.Context) {
		serveStaticFile(c, "apple-touch-icon-iphone-60x60.png", "image/png")
	})

	r.GET("/apple-touch-icon-ipad-76x76.png", func(c *gin.Context) {
		serveStaticFile(c, "apple-touch-icon-ipad-76x76.png", "image/png")
	})

	r.GET("/apple-touch-icon-iphone-retina-120x120.png", func(c *gin.Context) {
		serveStaticFile(c, "apple-touch-icon-iphone-retina-120x120.png", "image/png")
	})

	r.GET("/apple-touch-icon-ipad-retina-152x152.png", func(c *gin.Context) {
		serveStaticFile(c, "apple-touch-icon-ipad-retina-152x152.png", "image/png")
	})

	r.GET("/pwa-192x192.png", func(c *gin.Context) {
		serveStaticFile(c, "pwa-192x192.png", "image/png")
	})

	r.GET("/pwa-512x512.png", func(c *gin.Context) {
		serveStaticFile(c, "pwa-512x512.png", "image/png")
	})

	// Serve manifest.json
	r.GET("/manifest.json", func(c *gin.Context) {
		serveStaticFile(c, "manifest.json", "application/manifest+json; charset=utf-8")
	})

	// Serve service worker
	r.GET("/sw.js", func(c *gin.Context) {
		serveStaticFile(c, "sw.js", "text/javascript; charset=utf-8")
	})

	// Serve workbox files
	r.GET("/workbox-:hash.js", func(c *gin.Context) {
		serveStaticFile(c, c.Request.URL.Path[1:], "text/javascript; charset=utf-8")
	})

	// Serve assets directory
	r.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := strings.TrimPrefix(c.Param("filepath"), "/")
		fullPath := path.Join("assets", filepath)

		// Set content type based on file extension
		ext := strings.ToLower(path.Ext(filepath))
		var contentType string
		switch ext {
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js", ".mjs", ".tsx", ".ts":
			contentType = "text/javascript; charset=utf-8"
		case ".svg":
			contentType = "image/svg+xml"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".json":
			contentType = "application/json; charset=utf-8"
		case ".woff":
			contentType = "font/woff"
		case ".woff2":
			contentType = "font/woff2"
		default:
			contentType = "text/javascript; charset=utf-8"
		}

		serveStaticFile(c, fullPath, contentType)
	})

	// Serve index.html for root path and direct requests
	r.GET("/", serveIndex(dist))
	r.GET("/index.html", serveIndex(dist))

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
