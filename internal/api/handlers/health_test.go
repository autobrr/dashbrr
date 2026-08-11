// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/types"

	"github.com/gin-gonic/gin"

	testing_mocks "github.com/autobrr/dashbrr/internal/api/handlers/testing"
	"github.com/autobrr/dashbrr/internal/models"
)

// mockServiceHealthChecker implements models.ServiceHealthChecker interface for testing
type mockServiceHealthChecker struct {
	checkHealthFunc func(ctx context.Context, url, apiKey string) (models.ServiceHealth, int)
}

func (m *mockServiceHealthChecker) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	if m.checkHealthFunc != nil {
		return m.checkHealthFunc(ctx, url, apiKey)
	}
	return models.ServiceHealth{
		Status:      "healthy",
		LastChecked: time.Now(),
	}, http.StatusOK
}

// mockServiceCreator implements models.ServiceCreator interface for testing
type mockServiceCreator struct {
	createServiceFunc func(serviceType string) models.ServiceHealthChecker
}

func (m *mockServiceCreator) CreateService(serviceType string) models.ServiceHealthChecker {
	if m.createServiceFunc != nil {
		return m.createServiceFunc(serviceType)
	}
	return nil
}

func TestHealthHandler_CheckHealth(t *testing.T) {
	// Setup Gin in test mode
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		serviceID      string
		mockDBResponse func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error)
		mockHealth     func(ctx context.Context, url, apiKey string) (models.ServiceHealth, int)
		expectedCode   int
		expectedBody   gin.H
	}{
		{
			name:      "Service Not Found",
			serviceID: "nonexistent-service",
			mockDBResponse: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
				return nil, nil
			},
			expectedCode: http.StatusNotFound,
			expectedBody: gin.H{"error": "Service not found"},
		},
		{
			name:      "Database Error",
			serviceID: "error-service",
			mockDBResponse: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
				return nil, errors.New("database error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: gin.H{"error": "Failed to fetch service configuration"},
		},
		{
			name:      "Unsupported Service Type",
			serviceID: "invalid-service",
			mockDBResponse: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
				return &models.ServiceConfiguration{
					ID:         1,
					InstanceID: "invalid-service",
					URL:        "http://localhost:8080",
					APIKey:     "test-key",
				}, nil
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: gin.H{"error": "Unsupported service type: invalid"},
		},
		{
			name:      "Valid Service",
			serviceID: "autobrr-service",
			mockDBResponse: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
				return &models.ServiceConfiguration{
					ID:         1,
					InstanceID: "autobrr-service",
					URL:        "http://localhost:8080",
					APIKey:     "test-key",
				}, nil
			},
			mockHealth: func(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
				return models.ServiceHealth{
					Status:      "healthy",
					LastChecked: time.Now(),
				}, http.StatusOK
			},
			expectedCode: http.StatusOK,
			expectedBody: gin.H{"status": "healthy"},
		},
		{
			name:      "Service Not Configured",
			serviceID: "autobrr-empty",
			mockDBResponse: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
				return &models.ServiceConfiguration{
					ID:         2,
					InstanceID: "autobrr-empty",
					URL:        "",
					APIKey:     "test-key",
				}, nil
			},
			expectedCode: http.StatusOK,
			expectedBody: gin.H{
				"status":  "offline",
				"message": models.ErrNotConfigured,
				"serviceId": "autobrr-empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			mockDB := &testing_mocks.MockDB{
				FindServiceByFunc: tt.mockDBResponse,
			}

			// Create mock health checker
			mockChecker := &mockServiceHealthChecker{
				checkHealthFunc: tt.mockHealth,
			}

			// Create mock service creator that returns our mock checker for valid services
			mockCreator := &mockServiceCreator{
				createServiceFunc: func(serviceType string) models.ServiceHealthChecker {
					if serviceType == "autobrr" {
						return mockChecker
					}
					return nil
				},
			}

			// Create the handler with our mocks
			handler := NewHealthHandler(mockDB, mockCreator)

			// Setup the router
			r := gin.New()
			r.GET("/health/:service", handler.CheckHealth)

			// Create request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/health/"+tt.serviceID, nil)
			r.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Parse response
			var response gin.H
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response body: %v", err)
			}

			// Check response body
			for key, expectedValue := range tt.expectedBody {
				if actualValue, exists := response[key]; !exists || actualValue != expectedValue {
					t.Errorf("Expected response body to contain %s: %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestHealthHandler_CheckHealth_UsesStoredAPIKeyForURLValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &testing_mocks.MockDB{
		FindServiceByFunc: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
			if params.InstanceID != "radarr-1" {
				t.Fatalf("unexpected instance id: %s", params.InstanceID)
			}
			return &models.ServiceConfiguration{
				InstanceID: "radarr-1",
				URL:        "http://old-radarr",
				APIKey:     "stored-key",
			}, nil
		},
	}

	mockChecker := &mockServiceHealthChecker{
		checkHealthFunc: func(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
			if url != "http://new-radarr" {
				t.Fatalf("url = %q, want %q", url, "http://new-radarr")
			}
			if apiKey != "stored-key" {
				t.Fatalf("apiKey = %q, want %q", apiKey, "stored-key")
			}
			return models.ServiceHealth{
				Status:      "healthy",
				LastChecked: time.Now(),
			}, http.StatusOK
		},
	}

	mockCreator := &mockServiceCreator{
		createServiceFunc: func(serviceType string) models.ServiceHealthChecker {
			if serviceType == "radarr" {
				return mockChecker
			}
			return nil
		},
	}

	handler := NewHealthHandler(mockDB, mockCreator)
	r := gin.New()
	r.GET("/health/:service", handler.CheckHealth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/radarr-1?url=http://new-radarr", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealthHandler_CheckHealth_MissingAPIKeyForURLValidationWithoutStoredKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &testing_mocks.MockDB{
		FindServiceByFunc: func(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
			return nil, nil
		},
	}

	mockChecker := &mockServiceHealthChecker{
		checkHealthFunc: func(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
			t.Fatalf("checker should not be called")
			return models.ServiceHealth{}, http.StatusInternalServerError
		},
	}

	mockCreator := &mockServiceCreator{
		createServiceFunc: func(serviceType string) models.ServiceHealthChecker {
			if serviceType == "radarr" {
				return mockChecker
			}
			return nil
		},
	}

	handler := NewHealthHandler(mockDB, mockCreator)
	r := gin.New()
	r.GET("/health/:service", handler.CheckHealth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/radarr-1?url=http://new-radarr", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["message"] != "API key is required for this service type" {
		t.Fatalf("message = %q, want %q", response["message"], "API key is required for this service type")
	}
}
