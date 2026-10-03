// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/config"
)

const testIndex = `<head><meta charset="UTF-8" />
<base href="/" />
<link rel="manifest" href="manifest.json"></head>`

func newStaticRouter(t *testing.T, base config.BasePath) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	saved := DistDirFS
	DistDirFS = fstest.MapFS{
		"index.html":    {Data: []byte(testIndex)},
		"manifest.json": {Data: []byte(`{"scope":"./"}`)},
		"sw.js":         {Data: []byte(`// sw`)},
	}
	t.Cleanup(func() { DistDirFS = saved })

	r := gin.New()
	ServeStatic(r, base)
	return r
}

func get(t *testing.T, r http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return w
}

func TestServeStaticUnderBasePath(t *testing.T) {
	r := newStaticRouter(t, "/dashbrr")

	tests := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/dashbrr/", http.StatusOK, `<base href="/dashbrr/" />`},
		{"/dashbrr/login", http.StatusOK, `<base href="/dashbrr/" />`},
		{"/dashbrr/manifest.json", http.StatusOK, `"scope":"./"`},
		{"/dashbrr/sw.js", http.StatusOK, "// sw"},
		{"/dashbrr/api/unknown", http.StatusNotFound, ""},
		{"/manifest.json", http.StatusNotFound, ""},
		{"/other-app/", http.StatusNotFound, ""},
		{"/dashbrrx/", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := get(t, r, tt.path)
			if w.Code != tt.wantCode {
				t.Fatalf("GET %s = %d, want %d", tt.path, w.Code, tt.wantCode)
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("GET %s body = %q, want it to contain %q", tt.path, w.Body.String(), tt.wantBody)
			}
		})
	}

	if got := get(t, r, "/dashbrr/sw.js").Header().Get("Service-Worker-Allowed"); got != "" {
		t.Errorf("Service-Worker-Allowed = %q, want no header", got)
	}
}

func TestServeStaticAtRoot(t *testing.T) {
	r := newStaticRouter(t, "")

	for _, path := range []string{"/", "/login"} {
		w := get(t, r, path)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `<base href="/" />`) {
			t.Errorf("GET %s = %d %q, want 200 with the unchanged <base>", path, w.Code, w.Body.String())
		}
	}
	if w := get(t, r, "/api/unknown"); w.Code != http.StatusNotFound {
		t.Errorf("GET /api/unknown = %d, want 404", w.Code)
	}
}
