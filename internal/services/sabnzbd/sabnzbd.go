// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sabnzbd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	sabnzbdVersionCacheTTL = 1 * time.Hour
	sabnzbdFailedLimit     = 10
)

type ErrSabnzbd struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrSabnzbd) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("sabnzbd %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("sabnzbd %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("sabnzbd %s", e.Op)
}

func (e *ErrSabnzbd) Unwrap() error {
	return e.Err
}

type SabnzbdService struct {
	core.ServiceCore
}

func init() {
	models.NewSabnzbdService = NewSabnzbdService
}

func NewSabnzbdService() models.ServiceHealthChecker {
	service := &SabnzbdService{}
	service.Type = "sabnzbd"
	service.DisplayName = "SABnzbd"
	service.Description = "Monitor and manage your SABnzbd instance"
	service.DefaultURL = "http://localhost:8080"
	service.HealthEndpoint = "/api"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *SabnzbdService) GetHealthEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/api", strings.TrimRight(baseURL, "/"))
}

func (s *SabnzbdService) buildAPIURL(baseURL string, query url.Values) (string, error) {
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if baseURL == "" {
		return "", errors.New("URL is required")
	}

	endpoint, err := url.Parse(baseURL + "/api")
	if err != nil {
		return "", err
	}

	existing := endpoint.Query()
	for key, values := range query {
		for _, value := range values {
			existing.Add(key, value)
		}
	}
	endpoint.RawQuery = existing.Encode()

	return endpoint.String(), nil
}

func (s *SabnzbdService) getJSON(
	ctx context.Context,
	op string,
	baseURL string,
	apiKey string,
	query url.Values,
	out any,
) error {
	if strings.TrimSpace(baseURL) == "" {
		return &ErrSabnzbd{Op: op, Err: errors.New("URL is required")}
	}
	if strings.TrimSpace(apiKey) == "" {
		return &ErrSabnzbd{Op: op, Err: errors.New("API key is required")}
	}

	if query == nil {
		query = url.Values{}
	}
	query.Set("output", "json")
	query.Set("apikey", apiKey)

	requestURL, err := s.buildAPIURL(baseURL, query)
	if err != nil {
		return &ErrSabnzbd{Op: op, Err: fmt.Errorf("failed to build request URL: %w", err)}
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, requestURL, nil, nil)
	if err != nil {
		return &ErrSabnzbd{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrSabnzbd{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrSabnzbd{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &ErrSabnzbd{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
	}

	return nil
}

func (s *SabnzbdService) GetVersion(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := s.GetVersionFromCache(ctx, baseURL); version != "" && version != "true" {
		return version, nil
	}

	var envelope types.SabnzbdVersionEnvelope
	query := url.Values{}
	query.Set("mode", "version")
	if err := s.getJSON(ctx, "get_version", baseURL, apiKey, query, &envelope); err != nil {
		return "", err
	}

	version := strings.TrimSpace(envelope.Version)
	if version == "" {
		return "", &ErrSabnzbd{Op: "get_version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, sabnzbdVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache SABnzbd version")
	}

	return version, nil
}

func (s *SabnzbdService) GetQueue(ctx context.Context, baseURL, apiKey string) (types.SabnzbdQueue, error) {
	var envelope types.SabnzbdQueueEnvelope
	query := url.Values{}
	query.Set("mode", "queue")
	query.Set("start", "0")
	query.Set("limit", "10")
	if err := s.getJSON(ctx, "get_queue", baseURL, apiKey, query, &envelope); err != nil {
		return types.SabnzbdQueue{}, err
	}

	if envelope.Queue.Slots == nil {
		envelope.Queue.Slots = []types.SabnzbdQueueSlot{}
	}

	return envelope.Queue, nil
}

func (s *SabnzbdService) GetFailedHistory(ctx context.Context, baseURL, apiKey string, limit int) (types.SabnzbdHistory, error) {
	if limit <= 0 {
		limit = sabnzbdFailedLimit
	}

	var envelope types.SabnzbdHistoryEnvelope
	query := url.Values{}
	query.Set("mode", "history")
	query.Set("failed_only", "1")
	query.Set("limit", strconv.Itoa(limit))
	if err := s.getJSON(ctx, "get_failed_history", baseURL, apiKey, query, &envelope); err != nil {
		return types.SabnzbdHistory{}, err
	}

	history := envelope.History
	if history.Slots == nil {
		history.Slots = []types.SabnzbdHistorySlot{}
	}
	if history.NoOfSlots <= 0 {
		history.NoOfSlots = len(history.Slots)
	}

	return history, nil
}

func (s *SabnzbdService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.SabnzbdSummaryResponse, error) {
	var (
		queue      types.SabnzbdQueue
		history    types.SabnzbdHistory
		queueErr   error
		historyErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		queue, queueErr = s.GetQueue(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		history, historyErr = s.GetFailedHistory(ctx, baseURL, apiKey, sabnzbdFailedLimit)
	}()

	wg.Wait()

	if queueErr != nil && historyErr != nil {
		return types.SabnzbdSummaryResponse{}, fmt.Errorf(
			"all sabnzbd summary requests failed: queue: %v, history: %v",
			queueErr,
			historyErr,
		)
	}

	if queue.Slots == nil {
		queue.Slots = []types.SabnzbdQueueSlot{}
	}
	if history.Slots == nil {
		history.Slots = []types.SabnzbdHistorySlot{}
	}

	failedCount := history.NoOfSlots
	if failedCount <= 0 {
		failedCount = len(history.Slots)
	}

	return types.SabnzbdSummaryResponse{
		Queue:          queue,
		FailedCount:    failedCount,
		RecentFailures: history.Slots,
	}, nil
}

func (s *SabnzbdService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}
	if strings.TrimSpace(apiKey) == "" {
		return s.CreateHealthResponse(start, "error", "API key is required"), http.StatusBadRequest
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	var (
		version    string
		queue      types.SabnzbdQueue
		versionErr error
		queueErr   error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		version, versionErr = s.GetVersion(healthCtx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		queue, queueErr = s.GetQueue(healthCtx, baseURL, apiKey)
	}()

	wg.Wait()

	extras := map[string]any{
		"responseTime": time.Since(start).Milliseconds(),
	}
	if versionErr == nil && version != "" {
		extras["version"] = version
	}

	if queueErr != nil {
		return s.CreateHealthResponse(start, "error", fmt.Sprintf("Health check failed: %v", queueErr), extras), http.StatusOK
	}

	issues := make([]string, 0, 3)
	if strings.EqualFold(strings.TrimSpace(queue.Status), "paused") {
		issues = append(issues, "Paused")
	}

	warnings := parseSabnzbdInt(queue.HaveWarnings)
	if warnings > 0 {
		issues = append(issues, fmt.Sprintf("%d warning(s)", warnings))
	}

	if hasLowDiskSpace(queue.Diskspace1, queue.DiskspaceTotal1) {
		issues = append(issues, "Low incomplete disk space")
	}
	if hasLowDiskSpace(queue.Diskspace2, queue.DiskspaceTotal2) {
		issues = append(issues, "Low complete disk space")
	}

	if len(issues) > 0 {
		return s.CreateHealthResponse(start, "warning", strings.Join(issues, " • "), extras), http.StatusOK
	}

	return s.CreateHealthResponse(start, "online", "Healthy", extras), http.StatusOK
}

func parseSabnzbdInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsedInt, err := strconv.Atoi(value)
	if err == nil {
		return parsedInt
	}
	parsedFloat, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int(parsedFloat)
}

func hasLowDiskSpace(freeRaw, totalRaw string) bool {
	free, err := strconv.ParseFloat(strings.TrimSpace(freeRaw), 64)
	if err != nil || free <= 0 {
		return false
	}

	total, err := strconv.ParseFloat(strings.TrimSpace(totalRaw), 64)
	if err != nil || total <= 0 {
		return false
	}

	return (free / total) <= 0.05
}
