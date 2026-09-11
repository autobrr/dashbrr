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
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"

	"github.com/autobrr/dashbrr/internal/models"
)

// loginCacheTTL is how long a captured login value (cookie/token) is reused
// before we run the login step again.
const loginCacheTTL = 10 * time.Minute

// loginResult is what a login step produces: the value to inject (a cookie
// value or a captured JSON token), plus - when the value came from a cookie -
// the cookie's actual name. CaptureCookie may be a wildcard pattern (e.g.
// "*SID*" to match qBittorrent's port-suffixed "QBT_SID_8080"), so the actual
// matched name has to be remembered for injection; it must never be the
// pattern itself.
type loginResult struct {
	value      string
	cookieName string
}

type loginCacheEntry struct {
	value      string
	cookieName string
	expiresAt  time.Time
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

// ensureLogin returns a cached login result, or runs the login step and
// caches the result for loginCacheTTL. forceRefresh skips the cache read
// (used after a 401/403 invalidation) but still repopulates the cache on
// success. The cached cookieName (not just the pattern) is replayed on a
// cache hit so a later wildcard match doesn't have to be re-derived.
func (e *Engine) ensureLogin(ctx context.Context, baseURL string, cfg *models.CustomServiceConfig, apiKey string, forceRefresh bool) (loginResult, error) {
	key := loginCacheKey(baseURL, cfg)

	if !forceRefresh {
		loginCacheMu.Lock()
		entry, ok := loginCache[key]
		loginCacheMu.Unlock()
		if ok && time.Now().Before(entry.expiresAt) {
			return loginResult{value: entry.value, cookieName: entry.cookieName}, nil
		}
	}

	result, err := e.performLogin(ctx, baseURL, cfg, apiKey)
	if err != nil {
		return loginResult{}, err
	}

	loginCacheMu.Lock()
	loginCache[key] = loginCacheEntry{value: result.value, cookieName: result.cookieName, expiresAt: time.Now().Add(loginCacheTTL)}
	loginCacheMu.Unlock()

	return result, nil
}

// substituteLoginCredentials replaces the {{username}} and {{password}}
// placeholders in a login body template with the configured Auth
// credentials, regardless of Auth.Mode (a "none"-mode auth block can still
// carry credentials meant only for the login step, e.g. the qBittorrent
// preset username={{username}}&password={{password}}). A nil auth block
// substitutes empty strings. Never logged - the caller must not log the
// result.
//
// Each value is encoded for contentType before substitution: a credential
// containing a character special to the body's format (e.g. "&"/"=" in a
// form-urlencoded body, or '"'/backslash in a JSON body) would otherwise
// corrupt the request body or truncate the credential.
func substituteLoginCredentials(body, contentType string, auth *models.CustomAuthConfig) string {
	var username, password string
	if auth != nil {
		username = auth.Username
		password = auth.Password
	}

	encode := encodeForContentType(contentType)

	replacer := strings.NewReplacer(
		"{{username}}", encode(username),
		"{{password}}", encode(password),
	)
	return replacer.Replace(body)
}

// encodeForContentType returns the value-encoding function appropriate for
// a login body of contentType. Any content type other than the two form
// formats below (including empty/unspecified) is passed through raw, since
// there's no generic way to know how to escape an arbitrary body format.
func encodeForContentType(contentType string) func(string) string {
	mediaType := contentType
	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		mediaType = mediaType[:i]
	}
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "application/x-www-form-urlencoded":
		return url.QueryEscape
	case "application/json":
		return jsonStringEscape
	default:
		return func(s string) string { return s }
	}
}

// jsonStringEscape escapes s the way it would appear inside a JSON string
// literal (quotes, backslashes, control characters), without the
// surrounding quotes - the template already supplies those, e.g.
// {"password":"{{password}}"}.
func jsonStringEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	return string(b[1 : len(b)-1])
}

// matchCookieName reports whether a Set-Cookie name satisfies
// login.captureCookie. A pattern containing "*" is matched with Go's
// path.Match semantics (case-sensitive); a pattern without "*" is an exact
// match, same as before wildcards were supported.
func matchCookieName(pattern, name string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	matched, err := path.Match(pattern, name)
	return err == nil && matched
}

// performLogin runs cfg.Login once: sends the configured request, and
// captures either a named/wildcard-matched Set-Cookie value or a gjson path
// from the JSON body. Never logs the request body (raw or substituted) or
// the captured value.
func (e *Engine) performLogin(ctx context.Context, baseURL string, cfg *models.CustomServiceConfig, apiKey string) (loginResult, error) {
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
		if contentType == "" {
			contentType = "application/json"
		}
		bodyBytes = []byte(substituteLoginCredentials(login.Body, contentType, cfg.Auth))
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}

	loginURL := applyQueryParams(joinURL(baseURL, login.Path), query)

	resp, err := e.DoRequest(ctx, method, loginURL, headers, bodyBytes)
	if err != nil {
		return loginResult{}, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// If several Set-Cookie headers match the pattern, take the first.
	var capturedCookieName, capturedCookieValue string
	if login.CaptureCookie != "" {
		for _, c := range resp.Cookies() {
			if matchCookieName(login.CaptureCookie, c.Name) {
				capturedCookieName = c.Name
				capturedCookieValue = c.Value
				break
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return loginResult{}, fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return loginResult{}, fmt.Errorf("login request returned status %d", resp.StatusCode)
	}

	if login.CaptureJSONPath != "" {
		if val := gjson.GetBytes(body, login.CaptureJSONPath); val.Exists() {
			return loginResult{value: val.String()}, nil
		}
	}

	return loginResult{value: capturedCookieValue, cookieName: capturedCookieName}, nil
}
