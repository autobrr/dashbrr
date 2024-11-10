// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
)

var (
	// Global HTTP client pool
	httpClients sync.Map

	// Common errors
	ErrServiceNotConfigured = errors.New("service is not configured")
	ErrNilResponse          = errors.New("received nil response from server")
)

type ServiceCore struct {
	Type           string
	DisplayName    string
	Description    string
	DefaultURL     string
	ApiKey         string
	HealthEndpoint string
	cache          cache.Store
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
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
		Timeout: timeout,
	}

	// Store in pool
	httpClients.Store(timeout, client)
	return client
}

func (s *ServiceCore) initCache() error {
	if s.cache != nil {
		return nil
	}

	// Initialize cache using the cache package's initialization logic
	store, err := cache.InitCache()
	if err != nil {
		// If initialization fails, we'll still get a memory cache from InitCache
		// We can continue with the memory cache but should return the error
		// for logging purposes
		s.cache = store
		log.Warn().Err(err).Msg("Failed to initialize preferred cache, using memory cache")
		return err
	}

	s.cache = store
	return nil
}

// MakeRequestWithContext makes an HTTP request with the provided context and timeout
func (s *ServiceCore) MakeRequestWithContext(ctx context.Context, url string, apiKey string, headers map[string]string) (*http.Response, error) {
	if url == "" {
		log.Error().Msg("Service is not configured")
		return nil, ErrServiceNotConfigured
	}

	// Default timeout of 15 seconds if not specified in context
	timeout := 15 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

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
	req.Header.Set("User-Agent", "dashbrr/1.0")
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

	start := time.Now()

	// Get client with appropriate timeout
	client := getHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Request failed")
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

	// Store the response time in the response header
	resp.Header.Set("X-Response-Time", time.Since(start).String())

	return resp, nil
}

func (s *ServiceCore) MakeRequest(url string, apiKey string, headers map[string]string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.MakeRequestWithContext(ctx, url, apiKey, headers)
}

// ReadBody reads and returns the response body
func (s *ServiceCore) ReadBody(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, ErrNilResponse
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body")
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		var err error
		switch resp.StatusCode {
		case http.StatusBadGateway:
			err = errors.New("service unavailable (502 bad gateway)")
		case http.StatusServiceUnavailable:
			err = errors.New("service unavailable (503)")
		case http.StatusGatewayTimeout:
			err = errors.New("service timeout (504)")
		case http.StatusUnauthorized:
			err = errors.New("unauthorized access (401)")
		case http.StatusForbidden:
			err = errors.New("access forbidden (403)")
		case http.StatusNotFound:
			err = errors.New("endpoint not found (404)")
		default:
			if contentType != "application/json" {
				err = errors.New("service error")
			}
		}
		if err != nil {
			log.Error().Err(err).Int("status", resp.StatusCode).Str("content_type", contentType).Msg("Service error")
			return nil, err
		}
	}

	return body, nil
}

// GetVersionFromCache retrieves the version from cache
func (s *ServiceCore) GetVersionFromCache(baseURL string) string {
	if err := s.initCache(); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return ""
	}

	var version string
	cacheKey := "version:" + baseURL
	err := s.cache.Get(context.Background(), cacheKey, &version)
	if err != nil {
		// Cache miss is normal operation, no need to log it
		return ""
	}

	return version
}

// CacheVersion stores the version in cache with the specified TTL
func (s *ServiceCore) CacheVersion(baseURL, version string, ttl time.Duration) error {
	if err := s.initCache(); err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to initialize cache")
		return err
	}

	cacheKey := "version:" + baseURL
	if err := s.cache.Set(context.Background(), cacheKey, version, ttl); err != nil {
		log.Error().Err(err).Str("url", baseURL).Str("version", version).Msg("Failed to cache version")
		return err
	}

	return nil
}

// CreateHealthResponse creates a standardized health response
func (s *ServiceCore) CreateHealthResponse(lastChecked time.Time, status string, message string, extras ...map[string]interface{}) models.ServiceHealth {
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
	}

	return response
}

// GetCachedVersion attempts to get version from cache or fetches it if not found
func (s *ServiceCore) GetCachedVersion(ctx context.Context, baseURL, apiKey string, fetchVersion func(string, string) (string, error)) (string, error) {
	if err := s.initCache(); err != nil {
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
func (s *ServiceCore) ConcurrentRequest(requests []func() (interface{}, error)) []interface{} {
	var wg sync.WaitGroup
	results := make([]interface{}, len(requests))

	for i, request := range requests {
		wg.Add(1)
		go func(index int, req func() (interface{}, error)) {
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
