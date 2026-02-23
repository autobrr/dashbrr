// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
)

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Common errors
	ErrServiceNotConfigured = errors.New("service is not configured")
	ErrNilResponse          = errors.New("received nil response from server")
	ErrContextCanceled      = errors.New("context canceled")

	// Default timeouts
	DefaultTimeout     = 30 * time.Second // Increased from 15s to 30s
	DefaultLongTimeout = 60 * time.Second // Added for services that need longer timeouts
)

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (r *cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	if r.cancel != nil {
		r.once.Do(r.cancel)
	}
	return err
}

type ServiceCore struct {
	Type           string
	DisplayName    string
	Description    string
	DefaultURL     string
	ApiKey         string
	HealthEndpoint string
	Timeout        time.Duration // Added configurable timeout
	cache          cache.Store
	db             *database.DB
}

// SetDB sets the database instance for the service
func (s *ServiceCore) SetDB(db *database.DB) {
	s.db = db
}

// SetTimeout sets a custom timeout for the service
func (s *ServiceCore) SetTimeout(timeout time.Duration) {
	s.Timeout = timeout
}

func (s *ServiceCore) initCache(ctx context.Context) error {
	if s.cache != nil {
		return nil
	}

	// Get database directory from environment
	dataDir := filepath.Dir(os.Getenv("DASHBRR__DB_PATH"))
	if dataDir == "." {
		dataDir = "./data" // Default to ./data if not set
	}

	// Initialize cache config
	cfg := cache.Config{
		DataDir: dataDir,
	}

	// Use the global cache instance
	store, err := cache.InitCache(ctx, cfg)
	if err != nil {
		return err
	}
	s.cache = store
	if s.cache == nil {
		if err != nil {
			return err
		}
		return errors.New("cache init returned nil store")
	}
	return nil
}

// DoRequest makes an HTTP request with the provided context, method, and optional body.
// Uses the shared HTTP client pool + service-specific timeout.
func (s *ServiceCore) DoRequest(ctx context.Context, method string, url string, headers map[string]string, body []byte) (*http.Response, error) {
	if url == "" {
		log.Error().Msg("Service is not configured")
		return nil, ErrServiceNotConfigured
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Use service-specific timeout if set; otherwise default. If ctx already has a
	// deadline, rely on ctx cancellation (don't derive http.Client timeouts from
	// time.Until(deadline), which would create a new timeout key each request).
	timeout := DefaultTimeout
	if s.Timeout > 0 {
		timeout = s.Timeout
	}

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to create request")
		return nil, err
	}

	// Default headers
	buildinfo.AttachUserAgentHeader(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "keep-alive")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		log.Error().Err(err).
			Str("url", url).
			Dur("timeout", timeout).
			Msg("Request failed")
		return nil, err
	}
	if resp == nil {
		if cancel != nil {
			cancel()
		}
		log.Error().Str("url", url).Msg("Received nil response from server")
		return nil, ErrNilResponse
	}
	if cancel != nil {
		resp.Body = &cancelOnCloseReadCloser{
			ReadCloser: resp.Body,
			cancel:     cancel,
		}
	}

	// Redirect often indicates auth issues.
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		resp.Body.Close()
		err := errors.New("received redirect response, possible authentication issue")
		log.Error().Err(err).Str("url", url).Int("status", resp.StatusCode).Msg("Authentication error")
		return nil, err
	}

	resp.Header.Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	return resp, nil
}

func isJSONContentType(contentType string) bool {
	// Common: "application/json; charset=utf-8"
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
	}
	// Fallback for invalid headers.
	return strings.HasPrefix(contentType, "application/json") || strings.Contains(contentType, "+json")
}

// ReadBody reads and returns the response body
func (s *ServiceCore) ReadBody(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, ErrNilResponse
	}
	defer resp.Body.Close()

	// Read the entire body at once
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Debug().
				Err(err).
				Int("partial_bytes", len(body)).
				Msg("Response body read interrupted by context cancellation")
			return nil, ErrContextCanceled
		}

		log.Error().
			Err(err).
			Str("body", string(body)).
			Msg("Failed to read response body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		// Create error message with response body if available
		errMsg := fmt.Sprintf("status: %d", resp.StatusCode)
		if len(body) > 0 {
			errMsg = fmt.Sprintf("%s, body: %s", errMsg, string(body))
		}

		var err error
		switch resp.StatusCode {
		case http.StatusBadGateway:
			err = fmt.Errorf("service unavailable (502 bad gateway): %s", errMsg)
		case http.StatusServiceUnavailable:
			err = fmt.Errorf("service unavailable (503): %s", errMsg)
		case http.StatusGatewayTimeout:
			err = fmt.Errorf("service timeout (504): %s", errMsg)
		case http.StatusUnauthorized:
			err = fmt.Errorf("unauthorized access (401): %s", errMsg)
		case http.StatusForbidden:
			err = fmt.Errorf("access forbidden (403): %s", errMsg)
		case http.StatusNotFound:
			err = fmt.Errorf("endpoint not found (404): %s", errMsg)
		default:
			// Only create error if content type is not JSON
			if !isJSONContentType(contentType) {
				err = fmt.Errorf("service error: %s", errMsg)
			}
		}
		if err != nil {
			log.Error().
				Err(err).
				Int("status", resp.StatusCode).
				Str("content_type", contentType).
				Str("body", string(body)).
				Msg("Service error")
			return nil, err
		}
	}

	return body, nil
}

// GetVersionFromCache retrieves the version from cache
func (s *ServiceCore) GetVersionFromCache(ctx context.Context, baseURL string) string {
	if err := s.initCache(ctx); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return ""
	}

	var version string
	cacheKey := "version:" + baseURL
	err := s.cache.Get(ctx, cacheKey, &version)
	if err != nil {
		// Cache miss is normal operation, no need to log it
		return ""
	}

	return version
}

// GetUpdateStatusFromCacheWithFound retrieves the update status from cache and
// reports whether a value existed.
func (s *ServiceCore) GetUpdateStatusFromCacheWithFound(ctx context.Context, baseURL string) (bool, bool) {
	if err := s.initCache(ctx); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return false, false
	}

	var updateStatus string
	cacheKey := fmt.Sprintf("%s:update", baseURL)
	if err := s.cache.Get(ctx, cacheKey, &updateStatus); err == nil {
		return updateStatus == "true", true
	}

	// Legacy: older code incorrectly stored update status via CacheVersion(updateKey, ...),
	// which prefixes keys with "version:".
	var legacyStatus string
	legacyKey := "version:" + cacheKey
	if err := s.cache.Get(ctx, legacyKey, &legacyStatus); err == nil {
		return legacyStatus == "true", true
	}

	return false, false
}

// GetUpdateStatusFromCache retrieves the update status from cache.
func (s *ServiceCore) GetUpdateStatusFromCache(ctx context.Context, baseURL string) bool {
	updateStatus, _ := s.GetUpdateStatusFromCacheWithFound(ctx, baseURL)
	return updateStatus
}

// CacheUpdateStatus stores update availability in the dedicated update cache key.
func (s *ServiceCore) CacheUpdateStatus(ctx context.Context, baseURL string, updateAvailable bool, ttl time.Duration) error {
	if err := s.initCache(ctx); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return err
	}

	cacheKey := fmt.Sprintf("%s:update", baseURL)
	value := strconv.FormatBool(updateAvailable)
	if err := s.cache.Set(ctx, cacheKey, value, ttl); err != nil {
		log.Error().Err(err).Str("url", baseURL).Str("value", value).Msg("Failed to cache update status")
		return err
	}

	return nil
}

// CacheVersion stores the version in cache with the specified TTL
func (s *ServiceCore) CacheVersion(ctx context.Context, baseURL, version string, ttl time.Duration) error {
	if err := s.initCache(ctx); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return err
	}

	cacheKey := "version:" + baseURL
	if err := s.cache.Set(ctx, cacheKey, version, ttl); err != nil {
		log.Error().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache version")
		return err
	}

	return nil
}

// CreateHealthResponse creates a standardized health response
func (s *ServiceCore) CreateHealthResponse(lastChecked time.Time, status string, message string, extras ...map[string]any) models.ServiceHealth {
	response := models.ServiceHealth{
		Status:      status,
		LastChecked: lastChecked,
		Message:     message,
	}

	if len(extras) > 0 {
		if version, ok := extras[0]["version"].(string); ok {
			response.Version = version
		}
		if updateAvailable, ok := extras[0]["updateAvailable"].(bool); ok {
			response.UpdateAvailable = updateAvailable
		}
		if responseTime, ok := extras[0]["responseTime"].(int64); ok {
			response.ResponseTime = responseTime
		}
		if stats, ok := extras[0]["stats"].(map[string]any); ok {
			response.Stats = stats
		}
		if details, ok := extras[0]["details"].(map[string]any); ok {
			response.Details = details
		}
	}

	return response
}

// GetCachedVersion attempts to get version from cache or fetches it if not found
func (s *ServiceCore) GetCachedVersion(ctx context.Context, baseURL, apiKey string, fetchVersion func(string, string) (string, error)) (string, error) {
	if err := s.initCache(ctx); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Cache initialization failed")
		return "", err
	}

	cacheKey := "version:" + baseURL
	var version string

	// Try to get version from cache
	err := s.cache.Get(ctx, cacheKey, &version)
	if err == nil && version != "" {
		return version, nil
	}

	// If not in cache or error occurred, fetch it
	version, err = fetchVersion(baseURL, apiKey)
	if err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to fetch version")
		return "", err
	}

	// Cache the version for 1 hour
	if err := s.cache.Set(ctx, cacheKey, version, time.Hour); err != nil {
		log.Warn().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache version")
		return version, err
	}

	return version, nil
}

// ConcurrentRequest executes multiple requests concurrently and returns their results
func (s *ServiceCore) ConcurrentRequest(requests []func() (any, error)) []any {
	var wg sync.WaitGroup
	results := make([]any, len(requests))

	for i, request := range requests {
		wg.Add(1)
		go func(index int, req func() (any, error)) {
			defer wg.Done()
			if result, err := req(); err == nil {
				results[index] = result
			} else {
				log.Error().Err(err).Int("request_index", index).Msg("Concurrent request failed")
			}
		}(i, request)
	}

	wg.Wait()
	return results
}
