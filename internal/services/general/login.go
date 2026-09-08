// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"

	"github.com/autobrr/dashbrr/internal/models"
)

// loginCacheTTL is how long a captured login value (cookie/token) is reused
// before we run the login step again.
const loginCacheTTL = 10 * time.Minute

type loginCacheEntry struct {
	value     string
	expiresAt time.Time
}

// loginCache is keyed by (url, cfg-hash) rather than by Engine instance:
// the poller creates a fresh service/Engine value for every health check, so
// an instance-scoped cache would never be reused. The map is package-level
// and safe for concurrent use across all Engine instances.
var (
	loginCacheMu sync.Mutex
	loginCache   = map[string]loginCacheEntry{}
)

// loginCacheKey hashes the full config (not just the Login section) so that
// any config change - including to auth, health, or the login step itself -
// invalidates previously cached login values for that URL.
func loginCacheKey(baseURL string, cfg *models.CustomServiceConfig) string {
	h := sha256.New()
	h.Write([]byte(baseURL))
	if cfg != nil {
		if b, err := json.Marshal(cfg); err == nil {
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func invalidateLogin(baseURL string, cfg *models.CustomServiceConfig) {
	key := loginCacheKey(baseURL, cfg)
	loginCacheMu.Lock()
	delete(loginCache, key)
	loginCacheMu.Unlock()
}

// ensureLogin returns a cached login value, or runs the login step and caches
// the result for loginCacheTTL. forceRefresh skips the cache read (used after
// a 401/403 invalidation) but still repopulates the cache on success.
func (e *Engine) ensureLogin(ctx context.Context, baseURL string, cfg *models.CustomServiceConfig, apiKey string, forceRefresh bool) (string, error) {
	key := loginCacheKey(baseURL, cfg)

	if !forceRefresh {
		loginCacheMu.Lock()
		entry, ok := loginCache[key]
		loginCacheMu.Unlock()
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.value, nil
		}
	}

	value, err := e.performLogin(ctx, baseURL, cfg, apiKey)
	if err != nil {
		return "", err
	}

	loginCacheMu.Lock()
	loginCache[key] = loginCacheEntry{value: value, expiresAt: time.Now().Add(loginCacheTTL)}
	loginCacheMu.Unlock()

	return value, nil
}

// substituteLoginCredentials replaces the {{username}} and {{password}}
// placeholders in a login body template with the configured Auth
// credentials, regardless of Auth.Mode (a "none"-mode auth block can still
// carry credentials meant only for the login step, e.g. the qBittorrent
// preset username={{username}}&password={{password}}). A nil auth block
// substitutes empty strings. Never logged - the caller must not log the
// result.
func substituteLoginCredentials(body string, auth *models.CustomAuthConfig) string {
	var username, password string
	if auth != nil {
		username = auth.Username
		password = auth.Password
	}
	replacer := strings.NewReplacer(
		"{{username}}", username,
		"{{password}}", password,
	)
	return replacer.Replace(body)
}

// performLogin runs cfg.Login once: sends the configured request, and
// captures either a named Set-Cookie value or a gjson path from the JSON
// body. Never logs the request body (raw or substituted) or the captured
// value.
func (e *Engine) performLogin(ctx context.Context, baseURL string, cfg *models.CustomServiceConfig, apiKey string) (string, error) {
	login := cfg.Login

	method := login.Method
	if method == "" {
		if login.Body != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	headers, query, _ := buildAuthHeaders(cfg, apiKey)

	var bodyBytes []byte
	contentType := login.ContentType
	if login.Body != "" {
		bodyBytes = []byte(substituteLoginCredentials(login.Body, cfg.Auth))
		if contentType == "" {
			contentType = "application/json"
		}
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}

	loginURL := applyQueryParams(joinURL(baseURL, login.Path), query)

	resp, err := e.DoRequest(ctx, method, loginURL, headers, bodyBytes)
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	var capturedCookie string
	if login.CaptureCookie != "" {
		for _, c := range resp.Cookies() {
			if c.Name == login.CaptureCookie {
				capturedCookie = c.Value
				break
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login request returned status %d", resp.StatusCode)
	}

	if login.CaptureJSONPath != "" {
		if val := gjson.GetBytes(body, login.CaptureJSONPath); val.Exists() {
			return val.String(), nil
		}
	}

	return capturedCookie, nil
}
