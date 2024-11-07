// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type defaultFS struct {
	prefix string
	fs     fs.FS
}

type IndexParams struct {
	Title   string
	Version string
	BaseUrl string
}

var (
	//go:embed all:dist
	Dist embed.FS

	DistDirFS = MustSubFS(Dist, "dist")
)

func (fs defaultFS) Open(name string) (fs.File, error) {
	if fs.fs == nil {
		return os.Open(name)
	}
	return fs.fs.Open(name)
}

// MustSubFS creates sub FS from current filesystem or panic on failure.
func MustSubFS(currentFs fs.FS, fsRoot string) fs.FS {
	subFs, err := subFS(currentFs, fsRoot)
	if err != nil {
		panic(fmt.Errorf("can not create sub FS, invalid root given, err: %w", err))
	}
	return subFs
}

func subFS(currentFs fs.FS, root string) (fs.FS, error) {
	root = filepath.ToSlash(filepath.Clean(root))
	if dFS, ok := currentFs.(*defaultFS); ok {
		if !filepath.IsAbs(root) {
			root = filepath.Join(dFS.prefix, root)
		}
		return &defaultFS{
			prefix: root,
			fs:     os.DirFS(root),
		}, nil
	}
	return fs.Sub(currentFs, root)
}

// ServeStatic registers static file handlers with Gin
func ServeStatic(r *gin.Engine) {
	// Helper function to serve static files with proper headers
	serveStaticFile := func(c *gin.Context, filepath string, contentType string) {
		file, err := DistDirFS.Open(filepath)
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

		data, err := io.ReadAll(bufio.NewReader(file))
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

		reader := bytes.NewReader(data)
		http.ServeContent(c.Writer, c.Request, filepath, stat.ModTime(), reader)
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
	r.GET("/", serveIndex)
	r.GET("/index.html", serveIndex)

	// Handle all other routes
	r.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.AbortWithStatus(404)
			return
		}

		// For all other routes, serve index.html for client-side routing
		serveIndex(c)
	})
}

// serveIndex serves index.html with proper headers
func serveIndex(c *gin.Context) {
	file, err := DistDirFS.Open("index.html")
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
