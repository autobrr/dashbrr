// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/cache"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/domain"

	"github.com/rs/zerolog/log"
)

var (
	// Global HTTP client pool
	httpClients sync.Map

	// Common errors
	ErrServiceNotConfigured = errors.New("service is not configured")
	ErrNilResponse          = errors.New("received nil response from server")
	ErrContextCanceled      = errors.New("context canceled")

	// Default timeouts
	DefaultTimeout     = 30 * time.Second // Increased from 15s to 30s
	DefaultLongTimeout = 60 * time.Second // Added for services that need longer timeouts
)

type ServiceCore struct {
	InstanceID     string
	Type           domain.ServiceType
	DisplayName    string
	Description    string
	DefaultURL     string
	URL            string
	ApiKey         string
	HealthEndpoint string
	Timeout        time.Duration // Added configurable timeout
	cache          cache.Store
	db             *database.DB
}

// SetURL sets the URL for the service instance.
func (s *ServiceCore) SetURL(url string) {
	s.URL = url
}

// SetApiKey sets the API key for the service instance based on the provided key.
func (s *ServiceCore) SetApiKey(apiKey string) {
	s.ApiKey = apiKey
}

// SetDB sets the database instance for the service
func (s *ServiceCore) SetDB(db *database.DB) {
	s.db = db
}

// SetCache sets the cache instance for the service
func (s *ServiceCore) SetCache(cache cache.Store) {
	s.cache = cache
}

// SetTimeout sets a custom timeout for the service
func (s *ServiceCore) SetTimeout(timeout time.Duration) {
	s.Timeout = timeout
}

// getHTTPClient returns a client with the specified timeout
func getHTTPClient(timeout time.Duration) *http.Client {
	// Use the timeout as the key
	if client, ok := httpClients.Load(timeout); ok {
		return client.(*http.Client)
	}

	// Create new client if not found
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,               // Reduced from 100
			MaxIdleConnsPerHost: 2,                // Reduced from 10
			IdleConnTimeout:     30 * time.Second, // Reduced from 90s
			DisableKeepAlives:   false,
		},
		Timeout: timeout,
	}

	// Store in pool
	httpClients.Store(timeout, client)
	return client
}

// MakeRequestWithContext makes an HTTP request with the provided context and timeout
// TODO apikey from headers
func (s *ServiceCore) MakeRequestWithContext(ctx context.Context, url string, apiKey string, headers map[string]string) (*http.Response, error) {
	if url == "" {
		log.Error().Msg("Service is not configured")
		return nil, ErrServiceNotConfigured
	}

	// Use service-specific timeout if set, otherwise use context deadline or default
	//timeout := DefaultTimeout
	//if s.Timeout > 0 {
	//	timeout = s.Timeout
	//}
	//if deadline, ok := ctx.Deadline(); ok {
	//	timeout = time.Until(deadline)
	//}

	// Get method from headers if provided, default to GET
	method := http.MethodGet
	if m, ok := headers["method"]; ok {
		method = m
		delete(headers, "method") // Remove method from headers after using it
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to create request")
		return nil, err
	}

	// Set default headers
	buildinfo.AttachUserAgentHeader(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "keep-alive")

	if headers != nil {
		// Handle auth header first if present
		if authHeader, ok := headers["auth_header"]; ok {
			if authValue, ok := headers["auth_value"]; ok && authValue != "" {
				req.Header.Set(authHeader, authValue)
			}
		}

		// Set other headers
		for headerKey, headerValue := range headers {
			if headerKey != "auth_header" && headerKey != "auth_value" {
				req.Header.Set(headerKey, headerValue)
			}
		}
	}

	reqBody, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to dump request")
		//return nil, err
	}
	log.Debug().Str("url", url).Str("body", string(reqBody)).Msg("http request body")

	start := time.Now()

	// Get httpClient with appropriate timeout
	//client := getHTTPClient(timeout)

	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,               // Reduced from 100
			MaxIdleConnsPerHost: 2,                // Reduced from 10
			IdleConnTimeout:     30 * time.Second, // Reduced from 90s
			DisableKeepAlives:   false,
		},
		Timeout: DefaultTimeout,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("url", url).
			Dur("timeout", DefaultTimeout).
			Msg("Request failed")
		return nil, err
	}

	if resp == nil {
		log.Error().Str("url", url).Msg("Received nil response from server")
		return nil, ErrNilResponse
	}

	// Check if response is a redirect to a login page or similar
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		resp.Body.Close()
		err := errors.New("received redirect response, possible authentication issue")
		log.Error().Err(err).Str("url", url).Int("status", resp.StatusCode).Msg("Authentication error")
		return nil, err
	}

	// Store the response time in milliseconds
	resp.Header.Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))

	return resp, nil
}

func (s *ServiceCore) MakeRequest(url string, apiKey string, headers map[string]string) (*http.Response, error) {
	// Use service-specific timeout if set, otherwise use default
	timeout := DefaultTimeout
	if s.Timeout > 0 {
		timeout = s.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.MakeRequestWithContext(ctx, url, apiKey, headers)
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
		if errors.Is(err, context.Canceled) {
			if len(body) > 0 {
				log.Debug().
					Str("body", string(body)).
					Msg("Context canceled but partial response received")
				return body, nil
			}
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
			if contentType != "application/json" {
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

func (s *ServiceCore) GetDataFromCache(ctx context.Context, cacheKey string, data any) error {
	log.Debug().Str("url", s.URL).Str("instance", s.InstanceID).Str("cacheKey", cacheKey).Msg("Retrieving data from cache")

	err := s.cache.Get(ctx, cacheKey, &data)
	if err != nil {
		// Cache miss is normal operation, no need to log it
		return err
	}

	return nil
}

// GetVersionFromCache retrieves the version from cache
func (s *ServiceCore) GetVersionFromCache(baseURL string) string {
	var version string
	cacheKey := "version:" + s.InstanceID
	log.Debug().Str("url", baseURL).Str("instance", s.InstanceID).Str("cacheKey", cacheKey).Msg("Retrieving version from cache")

	err := s.cache.Get(context.Background(), cacheKey, &version)
	if err != nil {
		// Cache miss is normal operation, no need to log it
		return ""
	}

	return version
}

// GetVersionFromCacheCtx retrieves the version from cache
func (s *ServiceCore) GetVersionFromCacheCtx(ctx context.Context, baseURL string) string {
	log.Debug().Str("url", baseURL).Msg("Retrieving version from cache")

	var version string
	cacheKey := "version:" + s.InstanceID
	err := s.cache.Get(ctx, cacheKey, &version)
	if err != nil {
		// Cache miss is normal operation, no need to log it
		return ""
	}

	return version
}

// GetUpdateStatusFromCache retrieves the update status from cache
func (s *ServiceCore) GetUpdateStatusFromCache(baseURL string) bool {
	log.Trace().Str("url", baseURL).Str("instance", s.InstanceID).Msg("Retrieving update status from cache")

	var updateStatus string
	cacheKey := fmt.Sprintf("%s:update", s.InstanceID)
	err := s.cache.Get(context.Background(), cacheKey, &updateStatus)
	if err != nil {
		return false
	}

	return updateStatus == "true"
}

// CacheInstanceVersion stores the version in cache with the specified TTL
func (s *ServiceCore) CacheInstanceVersion(ctx context.Context, version string, ttl time.Duration) error {
	cacheKey := "version:" + s.InstanceID
	log.Trace().Str("url", s.URL).Str("instance", s.InstanceID).Str("version", version).Str("cacheKey", cacheKey).Msg("Caching instance version")

	if err := s.cache.Set(ctx, cacheKey, version, ttl); err != nil {
		log.Error().Err(err).Str("url", s.URL).Str("instance", s.InstanceID).Str("version", version).Msg("Failed to cache instance version")
		return err
	}

	return nil
}

// CacheVersion stores the version in cache with the specified TTL
func (s *ServiceCore) CacheVersion(ctx context.Context, baseURL, version string, ttl time.Duration) error {
	cacheKey := "version:" + s.InstanceID
	log.Trace().Str("url", baseURL).Str("instance", s.InstanceID).Str("version", version).Str("cacheKey", cacheKey).Msg("Caching version")

	if err := s.cache.Set(ctx, cacheKey, version, ttl); err != nil {
		log.Error().Err(err).Str("url", baseURL).Str("instance", s.InstanceID).Str("version", version).Msg("Failed to cache version")
		return err
	}

	return nil
}

func (s *ServiceCore) CacheData(ctx context.Context, cacheKey, data string, ttl time.Duration) error {
	//cacheKey := "version:" + s.InstanceID
	log.Trace().Str("url", s.URL).Str("instance", s.InstanceID).Str("data", data).Str("cacheKey", cacheKey).Msg("Caching data")

	if err := s.cache.Set(ctx, cacheKey, data, ttl); err != nil {
		log.Error().Err(err).Str("url", s.URL).Str("instance", s.InstanceID).Str("data", data).Msg("Failed to cache data")
		return err
	}

	return nil
}

func (s *ServiceCore) CacheHealth(ctx context.Context, cacheKey, data string, ttl time.Duration) error {
	//cacheKey := "version:" + s.InstanceID
	log.Trace().Str("url", s.URL).Str("instance", s.InstanceID).Str("data", data).Str("cacheKey", cacheKey).Msg("Caching health")

	if err := s.cache.Set(ctx, cacheKey, data, ttl); err != nil {
		log.Error().Err(err).Str("url", s.URL).Str("instance", s.InstanceID).Str("data", data).Msg("Failed to cache health")
		return err
	}

	return nil
}

// CreateHealthResponse creates a standardized health response
func (s *ServiceCore) CreateHealthResponse(lastChecked time.Time, status string, message string, extras ...map[string]interface{}) *domain.ServiceHealth {
	response := &domain.ServiceHealth{
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
		//if stats, ok := extras[0]["stats"].(map[string]interface{}); ok {
		//	response.Stats = stats
		//}
		//if details, ok := extras[0]["details"].(map[string]interface{}); ok {
		//	response.Details = details
		//}
	}

	return response
}

// GetCachedVersion attempts to get version from cache or fetches it if not found
// TODO check usage if this should be GetVersionFromCache instead for callers
func (s *ServiceCore) GetCachedVersion(ctx context.Context, baseURL, apiKey string, fetchVersion func(string, string) (string, error)) (string, error) {
	log.Trace().Str("url", baseURL).Msg("Retrieving version from cache")

	cacheKey := "version:" + s.InstanceID
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

//// ConcurrentRequest executes multiple requests concurrently and returns their results
//func (s *ServiceCore) ConcurrentRequest(requests []func() (interface{}, error)) []interface{} {
//	var wg sync.WaitGroup
//	results := make([]interface{}, len(requests))
//
//	for i, request := range requests {
//		wg.Add(1)
//		go func(index int, req func() (interface{}, error)) {
//			defer wg.Done()
//			if result, err := req(); err == nil {
//				results[index] = result
//			} else {
//				log.Error().Err(err).Int("request_index", index).Msg("Concurrent request failed")
//			}
//		}(i, request)
//	}
//
//	wg.Wait()
//	return results
//}
