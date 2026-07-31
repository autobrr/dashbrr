// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	// Dev UX: optionally proxy all non-API requests to a local Vite dev server.
	// This avoids stale `web/dist` embeds and keeps `http://localhost:8080` usable.
	if gin.Mode() == gin.DebugMode {
		if devServer := strings.TrimSpace(os.Getenv("DASHBRR_WEB_DEV_SERVER")); devServer != "" {
			target, err := url.Parse(devServer)
			if err == nil && target.Scheme != "" && target.Host != "" {
				proxy := httputil.NewSingleHostReverseProxy(target)
				originalDirector := proxy.Director
				proxy.Director = func(req *http.Request) {
					originalDirector(req)
					// Keep Vite happy behind the proxy.
					req.Host = target.Host
				}

				// Proxy everything except /api, which Gin serves.
				r.NoRoute(func(c *gin.Context) {
					if strings.HasPrefix(c.Request.URL.Path, "/api") {
						c.AbortWithStatus(http.StatusNotFound)
						return
					}
					proxy.ServeHTTP(c.Writer, c.Request)
				})

				r.GET("/", func(c *gin.Context) { proxy.ServeHTTP(c.Writer, c.Request) })
				return
			}
		}
	}

	// Serve any file that exists in the embedded dist. Reports whether it did,
	// so the caller can fall back to index.html for client-side routes.
	// ponytail: no per-file route list -- dist is the source of truth, so new
	// build outputs (registerSW.js, pattern.svg, ...) never need a Go change.
	serveStaticFile := func(c *gin.Context, name string) bool {
		if name == "index.html" {
			return false // serveIndex owns it, with no-store headers
		}
		file, err := DistDirFS.Open(name)
		if err != nil {
			return false
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			return false
		}

		// embed.FS and os.File both seek; ServeContent needs a ReadSeeker.
		seeker, ok := file.(io.ReadSeeker)
		if !ok {
			return false
		}

		switch {
		case name == "manifest.json":
			c.Header("Content-Type", "application/manifest+json; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
		case name == "sw.js" || name == "registerSW.js":
			c.Header("Cache-Control", "no-cache")
			c.Header("Service-Worker-Allowed", "/")
		case strings.HasSuffix(name, ".html"):
			// Unhashed pages (plex-auth-complete.html) must not cache for a year.
			c.Header("Cache-Control", "no-cache")
		default:
			c.Header("Cache-Control", "public, max-age=31536000")
		}
		c.Header("X-Content-Type-Options", "nosniff")

		// ServeContent picks the Content-Type from the extension when unset.
		http.ServeContent(c.Writer, c.Request, name, stat.ModTime(), seeker)
		return true
	}

	r.GET("/", serveIndex)

	r.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if serveStaticFile(c, name) {
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
	// NoRoute pre-sets 404; deep links are real pages, so reset it.
	c.Status(http.StatusOK)

	io.Copy(c.Writer, file)
}
