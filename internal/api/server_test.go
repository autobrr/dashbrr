// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/dashbrr/internal/api/session"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
)

func newTestHandler(t *testing.T, base config.BasePath) http.Handler {
	t.Helper()

	db, err := database.InitDBWithConfig(&database.Config{Driver: "sqlite", Path: t.TempDir() + "/server.db"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store := cache.NewMemoryStore(t.Context(), t.TempDir())
	t.Cleanup(func() { _ = store.Close() })

	cfg := config.DefaultConfig()
	cfg.Server.BasePath = base
	s := NewServer(cfg, db, store)
	h := s.Handler()
	t.Cleanup(s.pollerStop)
	return h
}

func request(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRoutesUnderBasePath(t *testing.T) {
	h := newTestHandler(t, "/dashbrr")

	tests := []struct {
		method, path string
		wantCode     int
		wantLocation string
	}{
		{http.MethodGet, "/health", http.StatusOK, ""},
		{http.MethodGet, "/dashbrr/health", http.StatusOK, ""},
		{http.MethodGet, "/", http.StatusTemporaryRedirect, "/dashbrr/"},
		{http.MethodGet, "/dashbrr", http.StatusTemporaryRedirect, "/dashbrr/"},
		{http.MethodGet, "/dashbrr/api/auth/config", http.StatusOK, ""},
		{http.MethodGet, "/api/auth/config", http.StatusNotFound, ""},
		{http.MethodGet, "/dashbrr/api/settings", http.StatusUnauthorized, ""},
		{http.MethodGet, "/api/settings", http.StatusNotFound, ""},
		{http.MethodGet, "/dashbrr/api/nope", http.StatusNotFound, ""},
		{http.MethodGet, "/other-app/", http.StatusNotFound, ""},
		{http.MethodGet, "/dashbrrx/", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := request(t, h, tt.method, tt.path, "")
			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.wantLocation, w.Header().Get("Location"))
		})
	}
}

func TestSessionCookieUnderBasePath(t *testing.T) {
	h := newTestHandler(t, "/dashbrr")

	w := request(t, h, http.MethodPost, "/dashbrr/api/auth/register",
		`{"username":"soup","email":"soup@example.invalid","password":"correct-Horse-battery-9"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = request(t, h, http.MethodPost, "/dashbrr/api/auth/login",
		`{"username":"soup","password":"correct-Horse-battery-9"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == session.CookieName {
			assert.Equal(t, "/dashbrr/", cookie.Path)
			return
		}
	}
	t.Fatal("login set no session cookie")
}

func TestRoutesAtRoot(t *testing.T) {
	h := newTestHandler(t, "")

	assert.Equal(t, http.StatusOK, request(t, h, http.MethodGet, "/health", "").Code)
	assert.Equal(t, http.StatusOK, request(t, h, http.MethodGet, "/api/auth/config", "").Code)
	assert.Equal(t, http.StatusUnauthorized, request(t, h, http.MethodGet, "/api/settings", "").Code)
	assert.NotEqual(t, http.StatusTemporaryRedirect, request(t, h, http.MethodGet, "/", "").Code)
}
