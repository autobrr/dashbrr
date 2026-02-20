// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package uptimekuma

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	uptimeKumaMetricsPath      = "/metrics"
	uptimeKumaScannerBufferMax = 2 * 1024 * 1024
	uptimeKumaAuthUser         = "dashbrr"
)

type ErrUptimeKuma struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrUptimeKuma) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("uptimekuma %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("uptimekuma %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("uptimekuma %s", e.Op)
}

func (e *ErrUptimeKuma) Unwrap() error {
	return e.Err
}

type UptimeKumaService struct {
	core.ServiceCore
}

func init() {
	models.NewUptimeKumaService = NewUptimeKumaService
}

func NewUptimeKumaService() models.ServiceHealthChecker {
	service := &UptimeKumaService{}
	service.Type = "uptimekuma"
	service.DisplayName = "Uptime Kuma"
	service.Description = "Monitor monitor health and incidents from Uptime Kuma"
	service.DefaultURL = "http://localhost:3001"
	service.HealthEndpoint = uptimeKumaMetricsPath
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *UptimeKumaService) GetHealthEndpoint(baseURL string) string {
	return fmt.Sprintf("%s%s", strings.TrimRight(baseURL, "/"), uptimeKumaMetricsPath)
}

func (s *UptimeKumaService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.UptimeKumaSummaryResponse, error) {
	metrics, err := s.getMetrics(ctx, baseURL, apiKey)
	if err != nil {
		return types.UptimeKumaSummaryResponse{}, err
	}

	summary, err := parseSummaryFromMetrics(metrics)
	if err != nil {
		return types.UptimeKumaSummaryResponse{}, &ErrUptimeKuma{Op: "parse_metrics", Err: err}
	}

	return summary, nil
}

func (s *UptimeKumaService) getMetrics(ctx context.Context, baseURL, apiKey string) (string, error) {
	endpoint, headers, err := buildMetricsRequest(baseURL, apiKey)
	if err != nil {
		return "", &ErrUptimeKuma{Op: "build_metrics_request", Err: err}
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return "", &ErrUptimeKuma{Op: "get_metrics", Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrUptimeKuma{Op: "get_metrics", HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", &ErrUptimeKuma{Op: "get_metrics", Err: fmt.Errorf("failed to read response: %w", err)}
	}

	return string(body), nil
}

func buildMetricsRequest(baseURL, apiKey string) (string, map[string]string, error) {
	trimmed := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if trimmed == "" {
		return "", nil, errors.New("URL is required")
	}

	endpoint, err := url.Parse(trimmed)
	if err != nil {
		return "", nil, fmt.Errorf("invalid URL: %w", err)
	}

	authHeader, err := resolveMetricsAuth(endpoint.User, apiKey)
	if err != nil {
		return "", nil, err
	}
	endpoint.User = nil
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + uptimeKumaMetricsPath
	endpoint.RawQuery = ""

	headers := map[string]string{"Accept": "text/plain"}
	if authHeader != "" {
		headers["Authorization"] = authHeader
	}

	return endpoint.String(), headers, nil
}

func resolveMetricsAuth(user *url.Userinfo, apiKey string) (string, error) {
	if token := strings.TrimSpace(apiKey); token != "" {
		return basicAuthHeader(uptimeKumaAuthUser, token), nil
	}
	if user == nil {
		return "", errors.New("API key is required (or include username:password in URL)")
	}

	username := strings.TrimSpace(user.Username())
	password, ok := user.Password()
	if username == "" || !ok || strings.TrimSpace(password) == "" {
		return "", errors.New("URL credentials must include username and password")
	}

	return basicAuthHeader(username, password), nil
}

func basicAuthHeader(username, password string) string {
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func parseSummaryFromMetrics(metrics string) (types.UptimeKumaSummaryResponse, error) {
	monitors := make(map[string]*types.UptimeKumaMonitor)

	scanner := bufio.NewScanner(strings.NewReader(metrics))
	scanner.Buffer(make([]byte, 64*1024), uptimeKumaScannerBufferMax)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, labels, value, ok := parseMetricLine(line)
		if !ok {
			continue
		}
		if name != "monitor_status" && name != "monitor_response_time" {
			continue
		}

		key := monitorKey(labels)
		if key == "" {
			continue
		}

		monitor := monitors[key]
		if monitor == nil {
			monitor = &types.UptimeKumaMonitor{ID: key, Status: "unknown"}
			monitors[key] = monitor
		}
		hydrateMonitorMeta(monitor, labels)

		switch name {
		case "monitor_status":
			monitor.Status = mapStatus(value)
		case "monitor_response_time":
			if value >= 0 {
				monitor.ResponseTimeMs = int64(value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return types.UptimeKumaSummaryResponse{}, err
	}

	list := make([]types.UptimeKumaMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		if strings.TrimSpace(monitor.Name) == "" {
			monitor.Name = "Monitor " + monitor.ID
		}
		list = append(list, *monitor)
	}

	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	return types.UptimeKumaSummaryResponse{Monitors: list}, nil
}

func parseMetricLine(line string) (string, map[string]string, float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", nil, 0, false
	}

	value, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return "", nil, 0, false
	}

	metric := fields[0]
	if !strings.Contains(metric, "{") {
		return metric, map[string]string{}, value, true
	}

	start := strings.IndexByte(metric, '{')
	end := strings.LastIndexByte(metric, '}')
	if start <= 0 || end <= start {
		return "", nil, 0, false
	}

	labels, err := parsePrometheusLabels(metric[start+1 : end])
	if err != nil {
		return "", nil, 0, false
	}

	return metric[:start], labels, value, true
}

func parsePrometheusLabels(raw string) (map[string]string, error) {
	labels := make(map[string]string)
	i := 0
	for i < len(raw) {
		for i < len(raw) && (raw[i] == ' ' || raw[i] == ',') {
			i++
		}
		if i >= len(raw) {
			break
		}

		keyStart := i
		for i < len(raw) && raw[i] != '=' {
			i++
		}
		if i >= len(raw) || i == keyStart {
			return nil, errors.New("invalid label key")
		}
		key := strings.TrimSpace(raw[keyStart:i])
		i++

		if i >= len(raw) || raw[i] != '"' {
			return nil, errors.New("invalid label value")
		}
		i++
		valueStart := i
		escaped := false
		for i < len(raw) {
			ch := raw[i]
			if escaped {
				escaped = false
				i++
				continue
			}
			if ch == '\\' {
				escaped = true
				i++
				continue
			}
			if ch == '"' {
				break
			}
			i++
		}
		if i >= len(raw) {
			return nil, errors.New("unterminated label value")
		}

		decoded, err := strconv.Unquote(`"` + raw[valueStart:i] + `"`)
		if err != nil {
			return nil, err
		}
		labels[key] = decoded
		i++
	}

	return labels, nil
}

func monitorKey(labels map[string]string) string {
	if id := strings.TrimSpace(labels["monitor_id"]); id != "" {
		return id
	}
	if name := strings.TrimSpace(labels["monitor_name"]); name != "" {
		return name
	}
	if hostname := strings.TrimSpace(labels["monitor_hostname"]); hostname != "" {
		return hostname
	}
	if target := strings.TrimSpace(labels["monitor_url"]); target != "" {
		return target
	}
	return ""
}

func hydrateMonitorMeta(monitor *types.UptimeKumaMonitor, labels map[string]string) {
	if monitor.ID == "" {
		monitor.ID = monitorKey(labels)
	}
	if monitor.Name == "" {
		monitor.Name = strings.TrimSpace(labels["monitor_name"])
	}
	if monitor.Type == "" {
		monitor.Type = strings.TrimSpace(labels["monitor_type"])
	}
	if monitor.URL == "" {
		monitor.URL = strings.TrimSpace(labels["monitor_url"])
	}
}

func mapStatus(value float64) string {
	switch int(value) {
	case 1:
		return "up"
	case 0:
		return "down"
	case 2:
		return "pending"
	case 3:
		return "maintenance"
	default:
		return "unknown"
	}
}

func summarizeMonitorStates(monitors []types.UptimeKumaMonitor) (total, up, down, pending, maintenance int) {
	total = len(monitors)
	for _, monitor := range monitors {
		switch monitor.Status {
		case "up":
			up++
		case "down":
			down++
		case "pending":
			pending++
		case "maintenance":
			maintenance++
		}
	}
	return total, up, down, pending, maintenance
}

func hasAuth(baseURL, apiKey string) bool {
	if strings.TrimSpace(apiKey) != "" {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.User == nil {
		return false
	}
	username := strings.TrimSpace(u.User.Username())
	password, ok := u.User.Password()
	return username != "" && ok && strings.TrimSpace(password) != ""
}

func (s *UptimeKumaService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}
	if !hasAuth(baseURL, apiKey) {
		return s.CreateHealthResponse(start, "error", "API key is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	summary, err := s.GetSummary(healthCtx, baseURL, apiKey)
	if err != nil {
		return s.CreateHealthResponse(start, "offline", fmt.Sprintf("Failed to connect: %v", err), map[string]interface{}{
			"responseTime": time.Since(start).Milliseconds(),
		}), http.StatusOK
	}

	total, up, down, pending, maintenance := summarizeMonitorStates(summary.Monitors)
	extras := map[string]interface{}{
		"responseTime": time.Since(start).Milliseconds(),
		"details": map[string]interface{}{
			"uptimekuma": map[string]interface{}{
				"total":       total,
				"up":          up,
				"down":        down,
				"pending":     pending,
				"maintenance": maintenance,
			},
		},
	}

	if down > 0 {
		return s.CreateHealthResponse(start, "warning", fmt.Sprintf("%d monitor(s) down", down), extras), http.StatusOK
	}
	if pending > 0 {
		return s.CreateHealthResponse(start, "warning", fmt.Sprintf("%d monitor(s) pending", pending), extras), http.StatusOK
	}
	if total == 0 {
		return s.CreateHealthResponse(start, "online", "Healthy - no monitors configured", extras), http.StatusOK
	}

	return s.CreateHealthResponse(start, "online", fmt.Sprintf("Healthy - %d monitor(s) up", up), extras), http.StatusOK
}
