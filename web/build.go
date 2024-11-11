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
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/config"
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

// BuildFrontend builds the frontend
func BuildFrontend(cfg *config.Config) error {
	// Run pnpm install
	installCmd := exec.Command("pnpm", "install")
	installCmd.Dir = "web"
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Run pnpm build
	buildCmd := exec.Command("pnpm", "build")
	buildCmd.Dir = "web"
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build frontend: %w", err)
	}

	return nil
}

// ServeStatic registers static file handlers with Gin
func ServeStatic(r *gin.Engine, cfg *config.Config) {
	baseURL := strings.TrimSuffix(cfg.Server.BaseURL, "/")

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

		// If this is index.html, inject the base URL
		if filepath == "index.html" {
			htmlStr := string(data)
			// Add base tag to head
			baseTag := fmt.Sprintf(`<base href="%s/">`, baseURL)
			htmlStr = strings.Replace(htmlStr, "<head>", "<head>"+baseTag, 1)
			data = []byte(htmlStr)
		}

		c.Header("Content-Type", contentType)
		if strings.Contains(filepath, "sw.js") || strings.Contains(filepath, "manifest.json") {
			c.Header("Cache-Control", "no-cache")
			if strings.Contains(filepath, "sw.js") {
				c.Header("Service-Worker-Allowed", baseURL)
			}
		} else {
			c.Header("Cache-Control", "public, max-age=31536000")
		}
		c.Header("X-Content-Type-Options", "nosniff")

		reader := bytes.NewReader(data)
		http.ServeContent(c.Writer, c.Request, filepath, stat.ModTime(), reader)
	}

	// Handle static files and SPA routes
	r.NoRoute(func(c *gin.Context) {
		// Don't handle API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			return
		}

		// Check if the request path starts with the base URL
		if !strings.HasPrefix(c.Request.URL.Path, baseURL) {
			c.AbortWithStatus(404)
			return
		}

		// Remove base URL from path to get the actual file path
		filepath := strings.TrimPrefix(c.Request.URL.Path, baseURL+"/")
		if filepath == "" {
			filepath = "index.html"
		}

		// Check if the file exists in our embedded filesystem
		if _, err := DistDirFS.Open(filepath); err != nil {
			// If file doesn't exist, serve index.html for client-side routing
			filepath = "index.html"
		}

		// Set content type based on file extension
		ext := strings.ToLower(path.Ext(filepath))
		var contentType string
		switch ext {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js", ".mjs":
			contentType = "application/javascript; charset=utf-8"
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
			contentType = "application/octet-stream"
		}

		serveStaticFile(c, filepath, contentType)
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

	// Read the file
	data, err := io.ReadAll(file)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Add base tag to head
	htmlStr := string(data)
	baseURL := strings.TrimSuffix(c.MustGet("config").(*config.Config).Server.BaseURL, "/")
	baseTag := fmt.Sprintf(`<base href="%s/">`, baseURL)
	htmlStr = strings.Replace(htmlStr, "<head>", "<head>"+baseTag, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlStr))
}
