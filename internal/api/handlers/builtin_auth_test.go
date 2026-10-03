// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/dashbrr/internal/api/session"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

const testPassword = "correct-Horse-battery-9"

func newBuiltinAuthRouter(t *testing.T, store cache.Store) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := database.InitDBWithConfig(&database.Config{Driver: "sqlite", Path: t.TempDir() + "/auth.db"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	h := NewBuiltinAuthHandler(db, store)
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/logout", h.Logout)
	r.GET("/verify", h.Verify)
	r.GET("/userinfo", h.GetUserInfo)
	return r
}

func newBuiltinMemoryStore(t *testing.T) cache.Store {
	t.Helper()
	store := cache.NewMemoryStore(t.Context(), t.TempDir())
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func serve(t *testing.T, r *gin.Engine, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func registerAndLogin(t *testing.T, r *gin.Engine) *http.Cookie {
	t.Helper()
	w := serve(t, r, http.MethodPost, "/register", `{"username":"soup","email":"soup@example.invalid","password":"`+testPassword+`"}`, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = serve(t, r, http.MethodPost, "/login", `{"username":"soup","password":"`+testPassword+`"}`, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == session.CookieName && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatal("login set no session cookie")
	return nil
}

func TestBuiltinAuth_RegisterOnlyFirstUser(t *testing.T) {
	r := newBuiltinAuthRouter(t, newBuiltinMemoryStore(t))
	registerAndLogin(t, r)

	w := serve(t, r, http.MethodPost, "/register", `{"username":"other","email":"other@example.invalid","password":"`+testPassword+`"}`, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestBuiltinAuth_LoginWrongPassword(t *testing.T) {
	r := newBuiltinAuthRouter(t, newBuiltinMemoryStore(t))
	registerAndLogin(t, r)

	w := serve(t, r, http.MethodPost, "/login", `{"username":"soup","password":"wrong-Password-1"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBuiltinAuth_SessionLifecycle(t *testing.T) {
	r := newBuiltinAuthRouter(t, newBuiltinMemoryStore(t))
	cookie := registerAndLogin(t, r)

	w := serve(t, r, http.MethodGet, "/verify", "", cookie)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = serve(t, r, http.MethodGet, "/userinfo", "", cookie)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"username":"soup"`)

	w = serve(t, r, http.MethodPost, "/logout", "", cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	w = serve(t, r, http.MethodGet, "/verify", "", cookie)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBuiltinAuth_CacheErrorGives503(t *testing.T) {
	store := new(MockStore)
	store.On("Get", mock.Anything, "session:token", mock.Anything).Return(errors.New("cache down"))
	r := newBuiltinAuthRouter(t, store)

	w := serve(t, r, http.MethodGet, "/verify", "", &http.Cookie{Name: session.CookieName, Value: "token"}) //nolint:gosec // request cookie; attributes don't apply
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestBuiltinAuth_UserInfoWithOIDCSession(t *testing.T) {
	store := newBuiltinMemoryStore(t)
	r := newBuiltinAuthRouter(t, store)
	require.NoError(t, store.Set(t.Context(), "session:oidc-token", types.SessionData{AuthType: "oidc"}, session.TTL))

	w := serve(t, r, http.MethodGet, "/userinfo", "", &http.Cookie{Name: session.CookieName, Value: "oidc-token"}) //nolint:gosec // request cookie; attributes don't apply
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "User not found")
}

// Each verifier must reject a session of the other login type. The web UI
// probes the OIDC verifier first and takes a 200 as an OIDC login.
func TestVerifiersRejectOtherAuthType(t *testing.T) {
	store := newBuiltinMemoryStore(t)
	require.NoError(t, store.Set(t.Context(), "session:builtin-token", types.SessionData{UserID: 1, AuthType: "builtin"}, session.TTL))
	require.NoError(t, store.Set(t.Context(), "session:oidc-token", types.SessionData{AuthType: "oidc"}, session.TTL))

	r := newBuiltinAuthRouter(t, store)
	oidc := NewAuthHandler(&types.AuthConfig{}, store)
	r.GET("/oidc/verify", oidc.VerifyToken)
	r.GET("/oidc/userinfo", oidc.UserInfo)

	for _, tc := range []struct {
		path  string
		token string
		want  int
	}{
		{path: "/oidc/verify", token: "builtin-token", want: http.StatusUnauthorized},
		{path: "/oidc/userinfo", token: "builtin-token", want: http.StatusUnauthorized},
		{path: "/verify", token: "oidc-token", want: http.StatusUnauthorized},
		{path: "/oidc/verify", token: "oidc-token", want: http.StatusOK},
		{path: "/oidc/userinfo", token: "oidc-token", want: http.StatusOK},
		{path: "/verify", token: "builtin-token", want: http.StatusOK},
	} {
		t.Run(tc.path+" "+tc.token, func(t *testing.T) {
			w := serve(t, r, http.MethodGet, tc.path, "", &http.Cookie{Name: session.CookieName, Value: tc.token}) //nolint:gosec // request cookie; attributes don't apply
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}
