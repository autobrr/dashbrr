// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestServiceTestUsesTheStoredKeyWhenTheBodyHasNone(t *testing.T) {
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

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/radarr-1", strings.NewReader(`{"url":"http://new-radarr"}`))
	serviceTestRouter(NewHealthHandler(mockDB, mockCreator)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServiceTestNeedsAKeyWhenNoneIsStored(t *testing.T) {
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

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/radarr-1", strings.NewReader(`{"url":"http://new-radarr"}`))
	serviceTestRouter(NewHealthHandler(mockDB, mockCreator)).ServeHTTP(w, req)

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

func loginURL(host string) string {
	return (&url.URL{Scheme: "http", User: url.UserPassword("user", "pass"), Host: host}).String()
}

func serviceTestRouter(h *HealthHandler) *gin.Engine {
	r := gin.New()
	r.GET("/health/:service", h.CheckHealth)
	r.POST("/health/:service", h.TestService)
	return r
}

func TestServiceTestReadsTheURLAndKeyFromTheBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &testing_mocks.MockDB{
		FindServiceByFunc: func(_ context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
			return &models.ServiceConfiguration{InstanceID: params.InstanceID, URL: "http://stored.test", APIKey: "stored-key"}, nil
		},
	}
	var gotURL, gotKey string
	creator := &mockServiceCreator{
		createServiceFunc: func(string) models.ServiceHealthChecker {
			return &mockServiceHealthChecker{
				checkHealthFunc: func(_ context.Context, url, apiKey string) (models.ServiceHealth, int) {
					gotURL, gotKey = url, apiKey
					return models.ServiceHealth{Status: "online"}, http.StatusOK
				},
			}
		},
	}
	r := serviceTestRouter(NewHealthHandler(db, creator))
	newURL := loginURL("new.test:6789")

	cases := []struct {
		name    string
		req     *http.Request
		wantURL string
		wantKey string
	}{
		{
			name:    "POST tests the URL and key of the body",
			req:     httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/nzbget-1", strings.NewReader(`{"url":"`+newURL+`","apiKey":"SECRET-KEY"}`)),
			wantURL: newURL,
			wantKey: "SECRET-KEY",
		},
		{
			name:    "GET ignores a URL and key in the query and tests the stored service",
			req:     httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/nzbget-1?"+url.Values{"url": {newURL}, "apiKey": {"SECRET-KEY"}}.Encode(), nil),
			wantURL: "http://stored.test",
			wantKey: "stored-key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotKey = "", ""
			w := httptest.NewRecorder()
			r.ServeHTTP(w, tc.req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
			}
			if gotURL != tc.wantURL || gotKey != tc.wantKey {
				t.Fatalf("checked %q with key %q, want %q with key %q", gotURL, gotKey, tc.wantURL, tc.wantKey)
			}
		})
	}
}

func TestServiceTestRejectsABodyWithoutAValidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	creator := &mockServiceCreator{
		createServiceFunc: func(string) models.ServiceHealthChecker {
			return &mockServiceHealthChecker{
				checkHealthFunc: func(context.Context, string, string) (models.ServiceHealth, int) {
					t.Fatal("checker should not run")
					return models.ServiceHealth{}, http.StatusOK
				},
			}
		},
	}
	r := serviceTestRouter(NewHealthHandler(&testing_mocks.MockDB{}, creator))

	for _, body := range []string{`{"apiKey":"k"}`, `{"url":"ftp://host"}`, `not json`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/sonarr-1", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", body, w.Code)
		}
	}
}

func TestServiceTestAcceptsALoginInTheURLInPlaceOfAKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	storedURL, newURL := loginURL("127.0.0.1:1"), loginURL("127.0.0.1:2")

	for _, serviceType := range []string{"nzbget", "uptimekuma"} {
		t.Run(serviceType, func(t *testing.T) {
			db := &testing_mocks.MockDB{
				FindServiceByFunc: func(_ context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
					return &models.ServiceConfiguration{InstanceID: params.InstanceID, URL: storedURL}, nil
				},
			}
			var gotURL string
			creator := &mockServiceCreator{
				createServiceFunc: func(string) models.ServiceHealthChecker {
					return &mockServiceHealthChecker{
						checkHealthFunc: func(_ context.Context, url, _ string) (models.ServiceHealth, int) {
							gotURL = url
							return models.ServiceHealth{Status: "online"}, http.StatusOK
						},
					}
				},
			}
			r := serviceTestRouter(NewHealthHandler(db, creator))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/"+serviceType+"-1", strings.NewReader(`{"url":"`+newURL+`"}`)))
			if w.Code != http.StatusOK || gotURL != newURL {
				t.Fatalf("POST: status = %d, checked %q, body %s", w.Code, gotURL, w.Body.String())
			}

			w = httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/"+serviceType+"-1", nil))
			if w.Code != http.StatusOK || gotURL != storedURL {
				t.Fatalf("GET: status = %d, checked %q, body %s", w.Code, gotURL, w.Body.String())
			}
		})
	}
}
