// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package nzbget

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
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
	nzbgetVersionCacheTTL = 1 * time.Hour
	nzbgetFailureLimit    = 10
	nzbgetQueueLimit      = 10
)

type ErrNzbget struct {
	Op       string
	Err      error
	HttpCode int
}

func (e *ErrNzbget) Error() string {
	if e.HttpCode > 0 {
		return fmt.Sprintf("nzbget %s: server returned %s (%d)", e.Op, http.StatusText(e.HttpCode), e.HttpCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("nzbget %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("nzbget %s", e.Op)
}

func (e *ErrNzbget) Unwrap() error {
	return e.Err
}

type rpcError struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

type rpcResponse[T any] struct {
	Result T         `json:"result"`
	Error  *rpcError `json:"error"`
}

type rpcRequest struct {
	NoCache int64  `json:"nocache"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type NzbgetService struct {
	core.ServiceCore
}

func init() {
	models.NewNzbgetService = NewNzbgetService
}

func NewNzbgetService() models.ServiceHealthChecker {
	service := &NzbgetService{}
	service.Type = "nzbget"
	service.DisplayName = "NZBGet"
	service.Description = "Monitor and manage your NZBGet instance"
	service.DefaultURL = "http://localhost:6789"
	service.HealthEndpoint = "/jsonrpc"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *NzbgetService) GetHealthEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/jsonrpc", strings.TrimRight(baseURL, "/"))
}

func (s *NzbgetService) GetVersion(ctx context.Context, baseURL, apiKey string) (string, error) {
	if version := s.GetVersionFromCache(ctx, baseURL); version != "" && version != "true" {
		return version, nil
	}

	var version string
	if err := s.callRPC(ctx, "version", baseURL, apiKey, nil, &version); err != nil {
		return "", err
	}

	version = strings.TrimSpace(version)
	if version == "" {
		return "", &ErrNzbget{Op: "version", Err: errors.New("version was empty")}
	}

	if err := s.CacheVersion(ctx, baseURL, version, nzbgetVersionCacheTTL); err != nil {
		log.Debug().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache NZBGet version")
	}

	return version, nil
}

func (s *NzbgetService) GetStatus(ctx context.Context, baseURL, apiKey string) (types.NzbgetStatus, error) {
	var status types.NzbgetStatus
	if err := s.callRPC(ctx, "status", baseURL, apiKey, nil, &status); err != nil {
		return types.NzbgetStatus{}, err
	}
	return status, nil
}

func (s *NzbgetService) GetQueue(ctx context.Context, baseURL, apiKey string, limit int) ([]types.NzbgetQueueItem, error) {
	var queue []types.NzbgetQueueItem
	if err := s.callRPC(ctx, "listgroups", baseURL, apiKey, []any{0}, &queue); err != nil {
		return nil, err
	}
	if queue == nil {
		queue = []types.NzbgetQueueItem{}
	}

	sortQueue(queue)
	if limit > 0 && len(queue) > limit {
		queue = queue[:limit]
	}
	return queue, nil
}

func (s *NzbgetService) GetFailedHistory(ctx context.Context, baseURL, apiKey string, limit int) ([]types.NzbgetHistoryItem, int, error) {
	var history []types.NzbgetHistoryItem
	if err := s.callRPC(ctx, "history", baseURL, apiKey, []any{false}, &history); err != nil {
		return nil, 0, err
	}
	if history == nil {
		history = []types.NzbgetHistoryItem{}
	}

	filtered := slices.DeleteFunc(history, func(item types.NzbgetHistoryItem) bool {
		return !isActionableHistoryStatus(item.Status)
	})
	sortHistory(filtered)

	total := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, total, nil
}

func (s *NzbgetService) GetSummary(ctx context.Context, baseURL, apiKey string) (types.NzbgetSummaryResponse, error) {
	var (
		status        types.NzbgetStatus
		statusErr     error
		queue         []types.NzbgetQueueItem
		queueErr      error
		failures      []types.NzbgetHistoryItem
		failedCount   int
		failedHistErr error
	)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		status, statusErr = s.GetStatus(ctx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		queue, queueErr = s.GetQueue(ctx, baseURL, apiKey, nzbgetQueueLimit)
	}()

	go func() {
		defer wg.Done()
		failures, failedCount, failedHistErr = s.GetFailedHistory(ctx, baseURL, apiKey, nzbgetFailureLimit)
	}()

	wg.Wait()

	if statusErr != nil && queueErr != nil && failedHistErr != nil {
		return types.NzbgetSummaryResponse{}, fmt.Errorf(
			"all nzbget summary requests failed: status: %v, queue: %v, history: %v",
			statusErr,
			queueErr,
			failedHistErr,
		)
	}

	if queue == nil {
		queue = []types.NzbgetQueueItem{}
	}
	if failures == nil {
		failures = []types.NzbgetHistoryItem{}
	}

	return types.NzbgetSummaryResponse{
		Status:         status,
		Queue:          queue,
		FailedCount:    failedCount,
		RecentFailures: failures,
	}, nil
}

func (s *NzbgetService) CheckHealth(ctx context.Context, baseURL, apiKey string) (models.ServiceHealth, int) {
	start := time.Now()

	if strings.TrimSpace(baseURL) == "" {
		return s.CreateHealthResponse(start, "error", "URL is required"), http.StatusBadRequest
	}
	if strings.TrimSpace(apiKey) == "" {
		if !urlHasCredentials(baseURL) {
			return s.CreateHealthResponse(start, "error", "Control password is required"), http.StatusBadRequest
		}
	}

	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	var (
		status     types.NzbgetStatus
		statusErr  error
		version    string
		versionErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		status, statusErr = s.GetStatus(healthCtx, baseURL, apiKey)
	}()

	go func() {
		defer wg.Done()
		version, versionErr = s.GetVersion(healthCtx, baseURL, apiKey)
	}()

	wg.Wait()

	extras := map[string]any{
		"responseTime": time.Since(start).Milliseconds(),
	}
	if versionErr == nil && version != "" {
		extras["version"] = version
	}

	if statusErr != nil {
		return s.CreateHealthResponse(start, "error", fmt.Sprintf("Health check failed: %v", statusErr), extras), http.StatusOK
	}

	warnings := make([]string, 0, 5)
	if status.DownloadPaused {
		warnings = append(warnings, "Download queue paused")
	}
	if status.PostPaused {
		warnings = append(warnings, "Post-processing paused")
	}
	if status.ScanPaused {
		warnings = append(warnings, "Scan paused")
	}
	if status.QuotaReached {
		warnings = append(warnings, "Quota reached")
	}

	remainingBytes := joinHiLo(status.RemainingSizeHi, status.RemainingSizeLo)
	freeBytes := joinHiLo(status.FreeDiskSpaceHi, status.FreeDiskSpaceLo)
	if remainingBytes > 0 && freeBytes > 0 && remainingBytes > freeBytes {
		warnings = append(warnings, "Insufficient free disk space for queue")
	}

	if len(warnings) > 0 {
		return s.CreateHealthResponse(start, "warning", strings.Join(warnings, " · "), extras), http.StatusOK
	}

	return s.CreateHealthResponse(start, "online", "Healthy", extras), http.StatusOK
}

func (s *NzbgetService) callRPC(
	ctx context.Context,
	op string,
	baseURL string,
	apiKey string,
	params []any,
	out any,
) error {
	requestURL, headers, err := s.rpcEndpointAndHeaders(op, baseURL, apiKey)
	if err != nil {
		return err
	}

	if params == nil {
		params = []any{}
	}

	payload, err := json.Marshal(rpcRequest{
		NoCache: time.Now().UnixMilli(),
		Method:  op,
		Params:  params,
	})
	if err != nil {
		return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to encode request: %w", err)}
	}

	resp, err := s.DoRequest(ctx, http.MethodPost, requestURL, headers, payload)
	if err != nil {
		return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to make request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrNzbget{Op: op, HttpCode: resp.StatusCode}
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	switch target := out.(type) {
	case *string:
		var parsed rpcResponse[string]
		if err := json.Unmarshal(body, &parsed); err != nil {
			return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		if parsed.Error != nil {
			return &ErrNzbget{Op: op, Err: rpcErrorString(parsed.Error)}
		}
		*target = parsed.Result
		return nil
	case *types.NzbgetStatus:
		var parsed rpcResponse[types.NzbgetStatus]
		if err := json.Unmarshal(body, &parsed); err != nil {
			return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		if parsed.Error != nil {
			return &ErrNzbget{Op: op, Err: rpcErrorString(parsed.Error)}
		}
		*target = parsed.Result
		return nil
	case *[]types.NzbgetQueueItem:
		var parsed rpcResponse[[]types.NzbgetQueueItem]
		if err := json.Unmarshal(body, &parsed); err != nil {
			return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		if parsed.Error != nil {
			return &ErrNzbget{Op: op, Err: rpcErrorString(parsed.Error)}
		}
		*target = parsed.Result
		return nil
	case *[]types.NzbgetHistoryItem:
		var parsed rpcResponse[[]types.NzbgetHistoryItem]
		if err := json.Unmarshal(body, &parsed); err != nil {
			return &ErrNzbget{Op: op, Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		if parsed.Error != nil {
			return &ErrNzbget{Op: op, Err: rpcErrorString(parsed.Error)}
		}
		*target = parsed.Result
		return nil
	default:
		return &ErrNzbget{Op: op, Err: errors.New("unsupported response target")}
	}
}

func (s *NzbgetService) rpcEndpointAndHeaders(op string, rawURL string, apiKey string) (string, map[string]string, error) {
	baseURL, username, password, err := parseCredentials(rawURL, apiKey)
	if err != nil {
		return "", nil, &ErrNzbget{Op: op, Err: err}
	}

	authValue := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	headers := map[string]string{
		"Authorization": "Basic " + authValue,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}

	return strings.TrimRight(baseURL, "/") + "/jsonrpc", headers, nil
}

func parseCredentials(rawURL, apiKey string) (baseURL, username, password string, err error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", "", "", errors.New("URL is required")
	}

	u, parseErr := url.Parse(trimmedURL)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", parseErr)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", "", "", errors.New("invalid URL")
	}

	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
		u.User = nil
	}

	if username == "" || password == "" {
		apiKey = strings.TrimSpace(apiKey)
		switch {
		case apiKey == "":
			return "", "", "", errors.New("control password is required")
		case strings.Contains(apiKey, ":"):
			parts := strings.SplitN(apiKey, ":", 2)
			username = strings.TrimSpace(parts[0])
			password = strings.TrimSpace(parts[1])
		default:
			username = "nzbget"
			password = apiKey
		}
	}

	if username == "" || password == "" {
		return "", "", "", errors.New("valid credentials are required")
	}

	return strings.TrimRight(u.String(), "/"), username, password, nil
}

func urlHasCredentials(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.User == nil {
		return false
	}
	username := strings.TrimSpace(u.User.Username())
	password, _ := u.User.Password()
	return username != "" && strings.TrimSpace(password) != ""
}

func rpcErrorString(err *rpcError) error {
	if err == nil {
		return errors.New("rpc error")
	}
	msg := strings.TrimSpace(err.Message)
	if msg == "" {
		msg = strings.TrimSpace(err.Name)
	}
	if msg == "" {
		if err.Code != 0 {
			msg = "RPC error " + strconv.Itoa(err.Code)
		} else {
			msg = "rpc error"
		}
	}
	return errors.New(msg)
}

func isActionableHistoryStatus(status string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	return strings.HasPrefix(normalized, "FAILURE/") || strings.HasPrefix(normalized, "WARNING/")
}

func sortQueue(queue []types.NzbgetQueueItem) {
	slices.SortFunc(queue, func(a, b types.NzbgetQueueItem) int {
		aActive := queueActiveRank(a.Status)
		bActive := queueActiveRank(b.Status)
		if aActive != bActive {
			return bActive - aActive
		}
		if a.RemainingSizeMB != b.RemainingSizeMB {
			if a.RemainingSizeMB > b.RemainingSizeMB {
				return -1
			}
			return 1
		}
		return strings.Compare(a.NZBName, b.NZBName)
	})
}

func sortHistory(history []types.NzbgetHistoryItem) {
	slices.SortFunc(history, func(a, b types.NzbgetHistoryItem) int {
		if a.HistoryTime == b.HistoryTime {
			return strings.Compare(a.Name, b.Name)
		}
		if a.HistoryTime > b.HistoryTime {
			return -1
		}
		return 1
	})
}

func queueActiveRank(status string) int {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DOWNLOADING", "FETCHING":
		return 3
	case "PAUSED":
		return 2
	case "QUEUED", "PP_QUEUED":
		return 1
	default:
		return 0
	}
}

func joinHiLo(hi, lo uint64) uint64 {
	return (hi << 32) | (lo & 0xffffffff)
}
