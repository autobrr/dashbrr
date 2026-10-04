// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package session owns the session: the cache key, the cookie, and the
// session token lookup. Builtin and OIDC logins use it in the same way.
package session

import (
	"cmp"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/internal/utils"
)

const (
	// CookieName has the app name as a prefix, so other services on the same
	// host cannot overwrite it. Cookies are scoped per host, not per port.
	CookieName = "dashbrr_user_session"

	// TTL is the session lifetime. It does not depend on the OIDC provider
	// token, which is discarded after login.
	TTL = 30 * 24 * time.Hour

	// keyPrefix must match the persistence filter in cache.MemoryStore.
	keyPrefix     = "session:"
	lookupTimeout = 5 * time.Second
)

// ErrNoToken means the request has no session token.
var ErrNoToken = errors.New("no session token")

type Manager struct {
	cache      cache.Store
	cookiePath string
}

// New makes a Manager. cookiePath is the path of the session cookie: "/" at
// the root, or the base path with a trailing slash, such as "/dashbrr/".
func New(store cache.Store, cookiePath string) *Manager {
	return &Manager{cache: store, cookiePath: cookiePath}
}

// Issue makes a token, stores data with ExpiresAt = now + TTL, and writes the cookie.
func (m *Manager) Issue(c *gin.Context, data types.SessionData) (string, error) {
	token, err := utils.GenerateSecureToken(32)
	if err != nil {
		return "", err
	}

	data.ExpiresAt = time.Now().Add(TTL)
	if err := m.cache.Set(c.Request.Context(), keyPrefix+token, data, TTL); err != nil {
		return "", err
	}

	m.setCookie(c, token, int(TTL.Seconds()))
	return token, nil
}

// Clear deletes the session, if there is one, and expires the cookie.
func (m *Manager) Clear(c *gin.Context) error {
	m.setCookie(c, "", -1)

	token, ok := requestToken(c)
	if !ok {
		return nil
	}
	if err := m.cache.Delete(c.Request.Context(), keyPrefix+token); err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		return err
	}
	return nil
}

// Load reads the cookie, then the Bearer header.
// It returns ErrNoToken when there is no token, and cache.ErrKeyNotFound when
// the session does not exist. When the lookup times out or the request is
// canceled, it returns the context error.
func (m *Manager) Load(c *gin.Context) (string, types.SessionData, error) {
	token, ok := requestToken(c)
	if !ok {
		return "", types.SessionData{}, ErrNoToken
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), lookupTimeout)
	defer cancel()

	var data types.SessionData
	if err := m.cache.Get(ctx, keyPrefix+token, &data); err != nil {
		return "", types.SessionData{}, cmp.Or(ctx.Err(), err)
	}
	return token, data, nil
}

func requestToken(c *gin.Context) (string, bool) {
	if token, err := c.Cookie(CookieName); err == nil && token != "" {
		return token, true
	}

	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return parts[1], true
}

func (m *Manager) setCookie(c *gin.Context, value string, maxAge int) {
	secure := c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie(CookieName, value, maxAge, m.cookiePath, "", secure, true)
}
