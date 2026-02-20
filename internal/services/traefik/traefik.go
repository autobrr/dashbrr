// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package traefik

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	traefikVersionCacheTTL  = 1 * time.Hour
	traefikVersionPath      = "/api/version"
	traefikOverviewPath     = "/api/overview"
	traefikRoutersPath      = "/api/http/routers"
	traefikMetricsPath      = "/metrics"
	traefikCertMetricName   = "traefik_tls_certs_not_after"
	traefikCertExpiringSoon = 30 * 24 * time.Hour
	maxIssueRouters         = 25
	maxTrackedCertEntries   = 25
)

var prometheusLabelRegexp = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"`)

type ErrTraefik struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrTraefik) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("traefik %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("traefik %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("traefik %s", e.Op)
}

func (e *ErrTraefik) Unwrap() error {
	return e.Err
}

type TraefikService struct {
	core.ServiceCore
}

func init() {
	models.NewTraefikService = NewTraefikService
}

func NewTraefikService() models.ServiceHealthChecker {
	service := &TraefikService{}
	service.Type = "traefik"
	service.DisplayName = "Traefik"
	service.Description = "Monitor Traefik routers and runtime health"
	service.DefaultURL = "http://localhost:8080"
	service.HealthEndpoint = traefikVersionPath
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *TraefikService) GetHealthEndpoint(baseURL string) string {
	endpoint, err := s.buildEndpoint(baseURL, traefikVersionPath, nil)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + traefikVersionPath
	}
	return endpoint
}

func (s *TraefikService) buildEndpoint(baseURL, path string, query url.Values) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if trimmed == "" {
		return "", errors.New("URL is required")
	}

	endpoint, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	if query != nil {
		endpoint.RawQuery = query.Encode()
	} else {
		endpoint.RawQuery = ""
	}
	return endpoint.String(), nil
}

func authHeaderFromAPIKey(apiKey string) string {
	token := strings.TrimSpace(apiKey)
	if token == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}

	if strings.Contains(token, ":") {
		encoded := base64.StdEncoding.EncodeToString([]byte(token))
		return "Basic " + encoded
	}

	return "Bearer " + token
}

func (s *TraefikService) getJSON(
	ctx context.Context,
	op,
	baseURL,
	apiKey,
	path string,
	query url.Values,
	out any,
) error {
	endpoint, err := s.buildEndpoint(baseURL, path, query)
	if err != nil {
		return &ErrTraefik{Op: op, Err: err}
	}

	headers := map[string]string{
		"Accept": "application/json",
	}
	if authHeader := authHeaderFromAPIKey(apiKey); authHeader != "" {
		headers["Authorization"] = authHeader
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrTraefik{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrTraefik{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return nil
}

func (s *TraefikService) GetVersion(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := strings.TrimSpace(s.GetVersionFromCache(ctx, baseURL)); version != "" && version != "true" {
		return version, nil
	}

	var payload types.TraefikVersionResponse
	if err := s.getJSON(ctx, "get_version", baseURL, apiKey, traefikVersionPath, nil, &payload); err != nil {
		return "", err
	}

	version := strings.TrimSpace(payload.Version)
	if version == "" {
		return "", &ErrTraefik{Op: "get_version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, traefikVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("failed to cache Traefik version")
	}

	return version, nil
}

func (s *TraefikService) GetOverview(ctx context.Context, baseURL, apiKey string) (types.TraefikOverviewResponse, error) {
	var payload types.TraefikOverviewResponse
	if err := s.getJSON(ctx, "get_overview", baseURL, apiKey, traefikOverviewPath, nil, &payload); err != nil {
		return types.TraefikOverviewResponse{}, err
	}
	return payload, nil
}

func (s *TraefikService) GetRoutersByStatus(
	ctx context.Context,
	baseURL,
	apiKey,
	status string,
) ([]types.TraefikRouter, error) {
	query := url.Values{}
	query.Set("status", status)
	query.Set("per_page", "100")

	var payload []types.TraefikRouter
	if err := s.getJSON(ctx, "get_http_routers", baseURL, apiKey, traefikRoutersPath, query, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = []types.TraefikRouter{}
	}
	return payload, nil
}

func mergeIssueRouters(warningRouters, disabledRouters []types.TraefikRouter) []types.TraefikRouter {
	merged := make([]types.TraefikRouter, 0, len(warningRouters)+len(disabledRouters))
	seen := make(map[string]struct{}, len(warningRouters)+len(disabledRouters))

	appendUnique := func(routers []types.TraefikRouter) {
		for _, router := range routers {
			key := strings.TrimSpace(router.Name) + "@" + strings.TrimSpace(router.Provider)
			if key == "@" {
				key = strings.TrimSpace(router.Rule) + "|" + strings.TrimSpace(router.Service)
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, router)
		}
	}

	appendUnique(disabledRouters)
	appendUnique(warningRouters)

	sort.SliceStable(merged, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(merged[i].Status))
		lj := strings.ToLower(strings.TrimSpace(merged[j].Status))
		if li != lj {
			return li < lj // disabled before warning
		}
		return strings.ToLower(merged[i].Name) < strings.ToLower(merged[j].Name)
	})

	if len(merged) > maxIssueRouters {
		return merged[:maxIssueRouters]
	}

	return merged
}

func parsePrometheusLabels(raw string) map[string]string {
	matches := prometheusLabelRegexp.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	labels := make(map[string]string, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		unquoted, err := strconv.Unquote(`"` + match[2] + `"`)
		if err != nil {
			unquoted = match[2]
		}
		labels[match[1]] = unquoted
	}
	return labels
}

func certificateStatus(expiresIn time.Duration) string {
	if expiresIn <= 0 {
		return "expired"
	}
	if expiresIn <= traefikCertExpiringSoon {
		return "expiring"
	}
	return "valid"
}

func parseTraefikCertificateMetrics(metricsBody string, now time.Time) (types.TraefikCertificateSummary, error) {
	scanner := bufio.NewScanner(strings.NewReader(metricsBody))
	certs := make([]types.TraefikCertificate, 0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, traefikCertMetricName) {
			continue
		}

		labelsRaw := ""
		valueRaw := ""
		if open := strings.IndexByte(line, '{'); open >= 0 {
			close := strings.LastIndexByte(line, '}')
			if close <= open {
				continue
			}
			labelsRaw = line[open+1 : close]
			valueRaw = strings.TrimSpace(line[close+1:])
		} else {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			valueRaw = parts[len(parts)-1]
		}

		notAfterFloat, err := strconv.ParseFloat(valueRaw, 64)
		if err != nil {
			continue
		}

		notAfterUnix := int64(notAfterFloat)
		notAfter := time.Unix(notAfterUnix, 0).UTC()
		expiresIn := notAfter.Sub(now)
		labels := parsePrometheusLabels(labelsRaw)

		sans := make([]string, 0)
		if sansRaw := strings.TrimSpace(labels["sans"]); sansRaw != "" {
			for _, san := range strings.Split(sansRaw, ",") {
				san = strings.TrimSpace(san)
				if san != "" {
					sans = append(sans, san)
				}
			}
		}

		certs = append(certs, types.TraefikCertificate{
			CommonName:       strings.TrimSpace(labels["cn"]),
			Serial:           strings.TrimSpace(labels["serial"]),
			SANs:             sans,
			NotAfter:         notAfter.Format(time.RFC3339),
			NotAfterUnix:     notAfterUnix,
			ExpiresInSeconds: int64(expiresIn.Seconds()),
			Status:           certificateStatus(expiresIn),
		})
	}
	if err := scanner.Err(); err != nil {
		return types.TraefikCertificateSummary{}, err
	}

	sort.SliceStable(certs, func(i, j int) bool {
		if certs[i].NotAfterUnix == certs[j].NotAfterUnix {
			return certs[i].CommonName < certs[j].CommonName
		}
		return certs[i].NotAfterUnix < certs[j].NotAfterUnix
	})

	summary := types.TraefikCertificateSummary{
		Total: len(certs),
	}

	if len(certs) > 0 {
		next := certs[0]
		summary.NextExpiry = next.NotAfter
		summary.NextExpiryUnix = next.NotAfterUnix
		summary.NextExpiryInSeconds = next.ExpiresInSeconds
	}

	for _, cert := range certs {
		switch cert.Status {
		case "expired":
			summary.Expired++
		case "expiring":
			summary.ExpiringSoon++
		}
	}

	if len(certs) > maxTrackedCertEntries {
		certs = certs[:maxTrackedCertEntries]
	}
	summary.Certificates = certs

	return summary, nil
}

func (s *TraefikService) metricsEndpoints(baseURL string) ([]string, error) {
	primary, err := s.buildEndpoint(baseURL, traefikMetricsPath, nil)
	if err != nil {
		return nil, err
	}

	endpoints := []string{primary}
	parsed, err := url.Parse(strings.TrimSpace(strings.TrimRight(baseURL, "/")))
	if err != nil {
		return endpoints, nil
	}

	host := parsed.Hostname()
	if host == "" || parsed.Port() == "9100" {
		return endpoints, nil
	}

	alt := *parsed
	alt.Host = net.JoinHostPort(host, "9100")
	alt.Path = traefikMetricsPath
	alt.RawQuery = ""
	altURL := alt.String()
	if altURL != primary {
		endpoints = append(endpoints, altURL)
	}

	return endpoints, nil
}

func (s *TraefikService) GetCertificateSummary(ctx context.Context, baseURL, apiKey string) (types.TraefikCertificateSummary, error) {
	endpoints, err := s.metricsEndpoints(baseURL)
	if err != nil {
		return types.TraefikCertificateSummary{}, &ErrTraefik{Op: "get_cert_metrics", Err: err}
	}

	headers := map[string]string{
		"Accept": "text/plain",
	}
	if authHeader := authHeaderFromAPIKey(apiKey); authHeader != "" {
		headers["Authorization"] = authHeader
	}

	var lastErr error
	now := time.Now().UTC()

	for _, endpoint := range endpoints {
		resp, reqErr := s.DoRequest(ctx, http.MethodGet, endpoint, headers, nil)
		if reqErr != nil {
			lastErr = &ErrTraefik{Op: "get_cert_metrics", Err: fmt.Errorf("request failed for %s: %w", endpoint, reqErr)}
			continue
		}

		body, readErr := s.ReadBody(resp)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = &ErrTraefik{Op: "get_cert_metrics", Err: fmt.Errorf("read failed for %s: %w", endpoint, readErr)}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = &ErrTraefik{Op: "get_cert_metrics", HttpCode: resp.StatusCode}
			continue
		}

		summary, parseErr := parseTraefikCertificateMetrics(string(body), now)
		if parseErr != nil {
			lastErr = &ErrTraefik{Op: "get_cert_metrics", Err: fmt.Errorf("parse failed for %s: %w", endpoint, parseErr)}
			continue
		}

		summary.MetricsURL = endpoint
		summary.MetricName = traefikCertMetricName
		return summary, nil
	}

	if lastErr == nil {
		lastErr = errors.New("no metrics endpoint candidates")
	}
	return types.TraefikCertificateSummary{}, lastErr
}

func (s *TraefikService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.TraefikSummaryResponse, error) {
	var (
		overview        types.TraefikOverviewResponse
		overviewErr     error
		warningRouters  []types.TraefikRouter
		warningErr      error
		disabledRouters []types.TraefikRouter
		disabledErr     error
		certs           types.TraefikCertificateSummary
		certsErr        error
	)

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		overview, overviewErr = s.GetOverview(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		warningRouters, warningErr = s.GetRoutersByStatus(ctx, baseURL, apiKey, "warning")
	}()

	go func() {
		defer wg.Done()
		disabledRouters, disabledErr = s.GetRoutersByStatus(ctx, baseURL, apiKey, "disabled")
	}()

	go func() {
		defer wg.Done()
		certs, certsErr = s.GetCertificateSummary(ctx, baseURL, apiKey)
	}()

	wg.Wait()

	if overviewErr != nil {
		return types.TraefikSummaryResponse{}, overviewErr
	}
	if warningErr != nil {
		log.Debug().Err(warningErr).Msg("traefik warning routers fetch failed")
	}
	if disabledErr != nil {
		log.Debug().Err(disabledErr).Msg("traefik disabled routers fetch failed")
	}
	if certsErr != nil {
		log.Debug().Err(certsErr).Msg("traefik cert metrics fetch failed")
	}

	summary := types.TraefikSummaryResponse{
		Overview:     overview,
		IssueRouters: mergeIssueRouters(warningRouters, disabledRouters),
	}
	if certsErr == nil {
		summary.Certificates = &certs
	}

	return summary, nil
}

func (s *TraefikService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	version, err := s.GetVersion(healthCtx, baseURL, apiKey)
	if err != nil {
		return s.CreateHealthResponse(start, "offline", fmt.Sprintf("Failed to connect: %v", err), map[string]interface{}{
			"responseTime": time.Since(start).Milliseconds(),
		}), http.StatusOK
	}

	return s.CreateHealthResponse(start, "online", "Healthy", map[string]interface{}{
		"responseTime": time.Since(start).Milliseconds(),
		"version":      version,
	}), http.StatusOK
}
