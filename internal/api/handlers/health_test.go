// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	testing_mocks "github.com/autobrr/dashbrr/internal/api/handlers/testing"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services"
)

// mockServiceHealthChecker implements models.ServiceHealthChecker interface for testing
type mockServiceHealthChecker struct {
	checkHealthFunc func(url, apiKey string) (models.ServiceHealth, int)
}

func (m *mockServiceHealthChecker) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	if m.checkHealthFunc != nil {
		return m.checkHealthFunc(url, apiKey)
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
		mockDBResponse func(string) (*models.ServiceConfiguration, error)
		mockHealth     func(url, apiKey string) (models.ServiceHealth, int)
		expectedCode   int
		expectedBody   gin.H
	}{
		{
			name:      "Service Not Found",
			serviceID: "nonexistent-service",
			mockDBResponse: func(id string) (*models.ServiceConfiguration, error) {
				return nil, nil
			},
			expectedCode: http.StatusNotFound,
			expectedBody: gin.H{"error": "Service not found"},
		},
		{
			name:      "Database Error",
			serviceID: "error-service",
			mockDBResponse: func(id string) (*models.ServiceConfiguration, error) {
				return nil, errors.New("database error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: gin.H{"error": "Failed to fetch service configuration"},
		},
		{
			name:      "Unsupported Service Type",
			serviceID: "invalid-service",
			mockDBResponse: func(id string) (*models.ServiceConfiguration, error) {
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
			mockDBResponse: func(id string) (*models.ServiceConfiguration, error) {
				return &models.ServiceConfiguration{
					ID:         1,
					InstanceID: "autobrr-service",
					URL:        "http://localhost:8080",
					APIKey:     "test-key",
				}, nil
			},
			mockHealth: func(url, apiKey string) (models.ServiceHealth, int) {
				return models.ServiceHealth{
					Status:      "healthy",
					LastChecked: time.Now(),
				}, http.StatusOK
			},
			expectedCode: http.StatusOK,
			expectedBody: gin.H{"status": "healthy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			mockDB := &testing_mocks.MockDB{
				GetServiceByInstanceIDFunc: tt.mockDBResponse,
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
			handler := NewHealthHandler(mockDB, services.NewHealthService(), mockCreator)

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
