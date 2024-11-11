package autobrr

import (
	"net/http"
	"time"
)

// Client represents an Autobrr service client
type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

// HealthCheckResponse represents the response from a health check
type HealthCheckResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// NewClient creates a new Autobrr service client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HealthCheck performs a health check on the Autobrr service
func (c *Client) HealthCheck() (*HealthCheckResponse, error) {
	// For now, return a mock health check response
	// In a real implementation, you'd make an actual HTTP request
	return &HealthCheckResponse{
		Status:  "OK",
		Version: "1.0.0", // This would be dynamically retrieved in a real implementation
	}, nil
}
