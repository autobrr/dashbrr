// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package general

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

// StatValue is a single extracted+formatted stat, as produced by Engine.FetchStats.
type StatValue struct {
	Label   string `json:"label"`
	Raw     any    `json:"raw"`
	Display string `json:"display"`
	Unit    string `json:"unit,omitempty"`
}

// ActionResult is the outcome of running a configured action via Engine.RunAction.
type ActionResult struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Engine drives a "general" service's health/stats/actions, optionally shaped
// by a models.CustomServiceConfig. GeneralService embeds Engine to satisfy
// models.ServiceHealthChecker.
type Engine struct {
	core.ServiceCore
}

// CheckHealth performs a health check for url. When cfg is nil (or has no
// Health section) it falls back to the original general-service behaviour:
// GET url, parse "status"/"message" from the JSON body, expose other
// top-level scalar fields under details.general.
func (e *Engine) CheckHealth(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (models.ServiceHealth, int) {
	startTime := time.Now()

	if rawURL == "" {
		return e.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	if cfg == nil {
		return e.legacyCheckHealth(ctx, rawURL, apiKey, startTime)
	}

	if cfg.Health == nil {
		return e.legacyCheckHealthWithAuth(ctx, rawURL, apiKey, cfg, startTime)
	}

	return e.checkHealthWithConfig(ctx, rawURL, apiKey, cfg, startTime)
}

func (e *Engine) legacyCheckHealth(ctx context.Context, rawURL, apiKey string, startTime time.Time) (models.ServiceHealth, int) {
	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	headers := make(map[string]string)
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	resp, err := e.DoRequest(healthCtx, http.MethodGet, rawURL, headers, nil)
	if err != nil {
		return e.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusServiceUnavailable
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return e.CreateHealthResponse(startTime, "error", fmt.Sprintf("Failed to read response: %v", err)), http.StatusInternalServerError
	}

	return legacyHealthResponse(e, startTime, body, responseTime), resp.StatusCode
}

// legacyCheckHealthWithAuth is used when a CustomServiceConfig is present but
// has no Health section configured: it keeps the original "GET url, parse
// status/message" shape, but drives authentication (and an optional login
// step) from cfg.Auth / cfg.Login instead of the plain apiKey-as-bearer rule.
func (e *Engine) legacyCheckHealthWithAuth(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, startTime time.Time) (models.ServiceHealth, int) {
	healthCtx, cancel := context.WithTimeout(ctx, e.timeoutFor(cfg))
	defer cancel()

	statusCode, body, err := e.doConfiguredRequest(healthCtx, rawURL, http.MethodGet, "", nil, cfg, apiKey)
	if err != nil {
		return e.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err)), http.StatusServiceUnavailable
	}

	responseTime := time.Since(startTime).Milliseconds()

	return legacyHealthResponse(e, startTime, body, responseTime), statusCode
}

func legacyHealthResponse(e *Engine, startTime time.Time, body []byte, responseTime int64) models.ServiceHealth {
	if status, message, fields, ok := legacyParseBody(body); ok {
		extras := map[string]any{"responseTime": responseTime}
		if len(fields) > 0 {
			extras["details"] = map[string]any{"general": fields}
		}
		return e.CreateHealthResponse(startTime, status, message, extras)
	}

	textResponse := strings.TrimSpace(string(body))
	extras := map[string]any{"responseTime": responseTime}

	if strings.EqualFold(textResponse, "ok") {
		return e.CreateHealthResponse(startTime, "online", "", extras)
	}

	return e.CreateHealthResponse(startTime, "error", "Unexpected response: "+textResponse, extras)
}

// legacyParseBody implements the original general-service JSON handling:
// a "status" string maps to online/offline/warning/unknown, "message" is
// surfaced as-is, and any other top-level scalar field is exposed for
// details.general (#87). Returns ok=false if body isn't a JSON object.
func legacyParseBody(body []byte) (status string, message string, fields map[string]any, ok bool) {
	var jsonResponse map[string]any
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return "", "", nil, false
	}

	status = "online"
	if statusVal, ok := jsonResponse["status"].(string); ok {
		switch strings.ToLower(statusVal) {
		case "healthy", "ok", "online":
			status = "online"
		case "unhealthy", "error", "offline":
			status = "offline"
		case "warning":
			status = "warning"
		default:
			status = "unknown"
		}
	}
	if messageVal, ok := jsonResponse["message"].(string); ok {
		message = messageVal
	}

	fields = make(map[string]any)
	for key, value := range jsonResponse {
		if key == "status" || key == "message" {
			continue
		}
		switch value.(type) {
		case string, float64, bool:
			fields[key] = value
		}
	}

	return status, message, fields, true
}

// checkHealthWithConfig implements the CustomServiceConfig-driven health
// check: configured method/path/body, gjson status/version extraction,
// ok/warn/offline mapping.
func (e *Engine) checkHealthWithConfig(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, startTime time.Time) (models.ServiceHealth, int) {
	healthCtx, cancel := context.WithTimeout(ctx, e.timeoutFor(cfg))
	defer cancel()

	method := cfg.Health.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyBytes []byte
	if cfg.Health.Body != "" {
		bodyBytes = []byte(cfg.Health.Body)
	}

	statusCode, body, err := e.doConfiguredRequest(healthCtx, rawURL, method, cfg.Health.Path, bodyBytes, cfg, apiKey)
	responseTime := time.Since(startTime).Milliseconds()
	if err != nil {
		return e.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect: %v", err), map[string]any{"responseTime": responseTime}), http.StatusServiceUnavailable
	}

	extras := map[string]any{"responseTime": responseTime}

	var healthStatus string
	if cfg.Health.StatusPath == "" {
		if statusCode >= 200 && statusCode < 300 {
			healthStatus = "online"
		} else {
			healthStatus = "offline"
		}
	} else {
		healthStatus = mapStatusValue(gjson.GetBytes(body, cfg.Health.StatusPath), cfg.Health.OKValues, cfg.Health.WarnValues)
	}

	if cfg.Health.VersionPath != "" {
		if v := gjson.GetBytes(body, cfg.Health.VersionPath); v.Exists() {
			extras["version"] = v.String()
		}
	}

	message := ""
	if healthStatus != "online" {
		message = fmt.Sprintf("status: %d", statusCode)
	}

	return e.CreateHealthResponse(startTime, healthStatus, message, extras), statusCode
}

// mapStatusValue implements the D2 status-mapping rule: value in OkValues ->
// online, in WarnValues -> warning. Comparisons are case-insensitive strings;
// gjson stringifies booleans/numbers for us.
//
// If either list is non-empty and the value matches neither, the result is
// offline - that's unchanged. But when BOTH lists are empty (statusPath is
// set with no ok/warn values configured), there's nothing to match against,
// so presence stands in for health: a non-empty extracted value is online,
// and a missing or empty value is offline. This matches Dashboarr's client
// and both projects' docs; previously the both-empty case always fell to
// offline, which contradicted them.
func mapStatusValue(val gjson.Result, okValues, warnValues []string) string {
	s := ""
	if val.Exists() {
		s = val.String()
	}

	for _, ok := range okValues {
		if strings.EqualFold(s, ok) {
			return "online"
		}
	}
	for _, warn := range warnValues {
		if strings.EqualFold(s, warn) {
			return "warning"
		}
	}

	if len(okValues) == 0 && len(warnValues) == 0 && val.Exists() && s != "" {
		return "online"
	}

	return "offline"
}

// FetchStats extracts cfg.Stats from the configured health response and
// formats each per its Format. Returns an empty map (no error) if cfg or
// cfg.Health is nil, or no stats are configured.
func (e *Engine) FetchStats(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (map[string]StatValue, error) {
	result := make(map[string]StatValue)
	if cfg == nil || cfg.Health == nil || len(cfg.Stats) == 0 {
		return result, nil
	}

	statsCtx, cancel := context.WithTimeout(ctx, e.timeoutFor(cfg))
	defer cancel()

	method := cfg.Health.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyBytes []byte
	if cfg.Health.Body != "" {
		bodyBytes = []byte(cfg.Health.Body)
	}

	_, body, err := e.doConfiguredRequest(statsCtx, rawURL, method, cfg.Health.Path, bodyBytes, cfg, apiKey)
	if err != nil {
		return nil, err
	}

	for _, stat := range cfg.Stats {
		val := gjson.GetBytes(body, stat.Path)
		result[stat.Label] = StatValue{
			Label:   stat.Label,
			Raw:     val.Value(),
			Display: formatStatValue(val, stat.Format),
			Unit:    stat.Unit,
		}
	}

	return result, nil
}

// RunAction executes the configured action identified by actionID. An
// unknown actionID (or a nil cfg) is an error.
func (e *Engine) RunAction(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, actionID string) (ActionResult, error) {
	if cfg == nil {
		return ActionResult{}, fmt.Errorf("unknown action %q: service has no configuration", actionID)
	}

	var action *models.CustomActionConfig
	for i := range cfg.Actions {
		if cfg.Actions[i].ID == actionID {
			action = &cfg.Actions[i]
			break
		}
	}
	if action == nil {
		return ActionResult{}, fmt.Errorf("unknown action %q", actionID)
	}

	actionCtx, cancel := context.WithTimeout(ctx, e.timeoutFor(cfg))
	defer cancel()

	method := action.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyBytes []byte
	if action.Body != "" {
		bodyBytes = []byte(action.Body)
	}

	statusCode, body, err := e.doConfiguredRequest(actionCtx, rawURL, method, action.Path, bodyBytes, cfg, apiKey)
	if err != nil {
		return ActionResult{}, err
	}

	return ActionResult{Status: statusCode, Body: string(body)}, nil
}

func (e *Engine) timeoutFor(cfg *models.CustomServiceConfig) time.Duration {
	if cfg != nil && cfg.TimeoutSeconds > 0 {
		return time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	if e.Timeout > 0 {
		return e.Timeout
	}
	return core.DefaultTimeout
}

// doConfiguredRequest applies auth + an optional login step, sends the
// request, and reads the body regardless of status code (callers, not
// core.ReadBody, decide what a given status means). On a 401/403 with a
// login step configured, it invalidates the cached login value and retries
// exactly once.
func (e *Engine) doConfiguredRequest(ctx context.Context, baseURL, method, path string, body []byte, cfg *models.CustomServiceConfig, apiKey string) (int, []byte, error) {
	hasLogin := cfg != nil && cfg.Login != nil && cfg.Login.Path != ""

	statusCode, respBody, err := e.attemptRequest(ctx, baseURL, method, path, body, cfg, apiKey, hasLogin, false)
	if err != nil {
		return 0, nil, err
	}

	if hasLogin && (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) {
		invalidateLogin(baseURL, cfg)
		if retryStatus, retryBody, retryErr := e.attemptRequest(ctx, baseURL, method, path, body, cfg, apiKey, hasLogin, true); retryErr == nil {
			return retryStatus, retryBody, nil
		}
	}

	return statusCode, respBody, nil
}

func (e *Engine) attemptRequest(ctx context.Context, baseURL, method, path string, body []byte, cfg *models.CustomServiceConfig, apiKey string, hasLogin, forceRelogin bool) (int, []byte, error) {
	headers, query, contentType := buildAuthHeaders(cfg, apiKey)

	if hasLogin {
		login, err := e.ensureLogin(ctx, baseURL, cfg, apiKey, forceRelogin)
		if err != nil {
			return 0, nil, fmt.Errorf("login failed: %w", err)
		}
		injectLogin(cfg.Login, login, headers, query)
	}

	if contentType != "" {
		headers["Content-Type"] = contentType
	} else if body != nil {
		headers["Content-Type"] = "application/json"
	}

	fullURL := applyQueryParams(joinURL(baseURL, path), query)

	resp, err := e.DoRequest(ctx, method, fullURL, headers, body)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, respBody, nil
}

// buildAuthHeaders returns the headers/query params for cfg.Auth, plus the
// content type it wants applied (currently always empty; content type is
// driven by the caller / login config). When cfg or cfg.Auth is nil it falls
// back to the pre-D2 rule: apiKey, if set, as a Bearer header.
func buildAuthHeaders(cfg *models.CustomServiceConfig, apiKey string) (headers map[string]string, query map[string]string, contentType string) {
	headers = make(map[string]string)
	query = make(map[string]string)

	if cfg == nil || cfg.Auth == nil {
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		return headers, query, ""
	}

	value := apiKey
	if value == "" {
		value = cfg.Auth.Token
	}

	switch cfg.Auth.Mode {
	case "", "none":
		// no auth applied
	case "header":
		if cfg.Auth.HeaderName != "" && value != "" {
			headers[cfg.Auth.HeaderName] = value
		}
	case "query":
		if cfg.Auth.QueryParam != "" && value != "" {
			query[cfg.Auth.QueryParam] = value
		}
	case "basic":
		headers["Authorization"] = basicAuthHeader(cfg.Auth.Username, cfg.Auth.Password)
	case "bearer":
		if value != "" {
			headers["Authorization"] = "Bearer " + value
		}
	}

	return headers, query, ""
}

func basicAuthHeader(username, password string) string {
	creds := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// injectLogin applies a captured login result into headers/query per
// login.InjectAs. Never logs the value.
//
// For InjectAs "cookie" with no InjectName configured, the cookie name comes
// from result.cookieName - the actual Set-Cookie name matched during login,
// never login.CaptureCookie itself (which may be a wildcard pattern like
// "*SID*" and would produce a broken "Cookie: *SID*=..." header).
func injectLogin(login *models.CustomLoginConfig, result loginResult, headers, query map[string]string) {
	if login == nil || result.value == "" {
		return
	}

	switch login.InjectAs {
	case "header":
		if login.InjectName != "" {
			headers[login.InjectName] = result.value
		}
	case "query":
		if login.InjectName != "" {
			query[login.InjectName] = result.value
		}
	case "cookie":
		name := login.InjectName
		if name == "" {
			name = result.cookieName
		}
		if name != "" {
			headers["Cookie"] = name + "=" + result.value
		}
	case "bearer":
		headers["Authorization"] = "Bearer " + result.value
	}
}

// joinURL appends path to baseURL, tolerating either side's slashes. An
// absolute path (http(s)://...) is returned unchanged.
func joinURL(baseURL, path string) string {
	if path == "" {
		return baseURL
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(baseURL, "/") + path
}

// applyQueryParams adds params to rawURL's query string, preserving any that
// are already present.
func applyQueryParams(rawURL string, params map[string]string) string {
	if len(params) == 0 {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String()
}
