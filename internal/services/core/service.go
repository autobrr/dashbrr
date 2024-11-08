// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/cache"
)

var (
	// Global HTTP client pool
	httpClients sync.Map
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

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "127.0.0.1"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	c, err := cache.NewCache(redisAddr)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %v", err)
	}

	s.cache = c
	return nil
}

// MakeRequestWithContext makes an HTTP request with the provided context and timeout
func (s *ServiceCore) MakeRequestWithContext(ctx context.Context, url string, apiKey string, headers map[string]string) (*http.Response, error) {
	if url == "" {
		return nil, fmt.Errorf("service is not configured")
	}

	// Default timeout of 15 seconds if not specified in context
	timeout := 15 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
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
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("received nil response from server")
	}

	// Check if response is a redirect to a login page or similar
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		resp.Body.Close()
		return nil, fmt.Errorf("received redirect response (status %d), possible authentication issue", resp.StatusCode)
	}

	// Store the response time in the response header
	resp.Header.Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))

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
		return nil, fmt.Errorf("nil response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusBadGateway:
			return nil, fmt.Errorf("service unavailable (502 bad gateway)")
		case http.StatusServiceUnavailable:
			return nil, fmt.Errorf("service unavailable (503)")
		case http.StatusGatewayTimeout:
			return nil, fmt.Errorf("service timeout (504)")
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("unauthorized access (401)")
		case http.StatusForbidden:
			return nil, fmt.Errorf("access forbidden (403)")
		case http.StatusNotFound:
			return nil, fmt.Errorf("endpoint not found (404)")
		default:
			if contentType != "application/json" {
				return nil, fmt.Errorf("service error (status %d)", resp.StatusCode)
			}
		}
	}

	return body, nil
}

// GetVersionFromCache retrieves the version from cache
func (s *ServiceCore) GetVersionFromCache(baseURL string) string {
	if err := s.initCache(); err != nil {
		return ""
	}

	var version string
	cacheKey := fmt.Sprintf("version:%s", baseURL)
	err := s.cache.Get(context.Background(), cacheKey, &version)
	if err != nil {
		return ""
	}

	return version
}

// CacheVersion stores the version in cache with the specified TTL
func (s *ServiceCore) CacheVersion(baseURL, version string, ttl time.Duration) error {
	if err := s.initCache(); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("version:%s", baseURL)
	return s.cache.Set(context.Background(), cacheKey, version, ttl)
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
		return "", fmt.Errorf("cache initialization failed: %v", err)
	}

	cacheKey := fmt.Sprintf("version:%s", baseURL)
	var version string

	// Try to get version from cache
	err := s.cache.Get(ctx, cacheKey, &version)
	if err == nil && version != "" {
		return version, nil
	}

	// If not in cache or error occurred, fetch it
	version, err = fetchVersion(baseURL, apiKey)
	if err != nil {
		return "", err
	}

	// Cache the version for 1 hour
	if err := s.cache.Set(ctx, cacheKey, version, time.Hour); err != nil {
		return version, fmt.Errorf("failed to cache version: %v", err)
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
			}
		}(i, request)
	}

	wg.Wait()
	return results
}
