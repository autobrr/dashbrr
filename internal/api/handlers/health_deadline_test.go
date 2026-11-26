// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	testing_mocks "github.com/autobrr/dashbrr/internal/api/handlers/testing"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/catalog"
	"github.com/autobrr/dashbrr/internal/types"
)

func postServiceTest(t *testing.T, h *HealthHandler, serviceType, serviceURL string) (*httptest.ResponseRecorder, time.Duration) {
	t.Helper()
	apiKey := "test-key"
	if serviceType == "netbird" {
		apiKey = "nbp_test"
	}
	body, err := json.Marshal(serviceTestRequest{URL: serviceURL, APIKey: apiKey})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/"+serviceType+"-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	start := time.Now()
	serviceTestRouter(h).ServeHTTP(w, req)
	return w, time.Since(start)
}

const callsFixedPublicHost = "tailscale"

func TestServiceTestFailsEveryTypeWithoutASessionStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stop := make(chan struct{})
	servers := map[string]http.HandlerFunc{
		"hung service": func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-stop:
			}
		},
		"service rejects the key": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
		},
	}

	urls := map[string]string{}
	for name, serve := range servers {
		server := httptest.NewServer(serve)
		t.Cleanup(server.Close)
		urls[name] = server.URL
	}
	t.Cleanup(func() { close(stop) })

	for name, serverURL := range urls {
		for _, st := range catalog.All() {
			if st.Name == callsFixedPublicHost {
				continue
			}
			t.Run(name+"/"+st.Name, func(t *testing.T) {
				db := &testing_mocks.MockDB{
					FindServiceByFunc: func(context.Context, types.FindServiceParams) (*models.ServiceConfiguration, error) {
						return nil, nil
					},
				}
				h := NewHealthHandler(db, catalog.Registry{})
				h.timeout = 300 * time.Millisecond

				w, elapsed := postServiceTest(t, h, st.Name, serverURL)

				if elapsed > 3*time.Second {
					t.Fatalf("answered after %s, want within the %s deadline", elapsed, h.timeout)
				}
				var body struct {
					Status  string `json:"status"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode %q: %v", w.Body.String(), err)
				}
				switch w.Code {
				case http.StatusBadGateway, http.StatusGatewayTimeout:
				case http.StatusOK:
					if body.Status != "error" && body.Status != "offline" {
						t.Fatalf("status = 200 %q, want a failed test", body.Status)
					}
				default:
					t.Fatalf("status = %d, want 502, 504, or 200 with a failed status", w.Code)
				}
				if body.Message == "" {
					t.Fatalf("no message in %s", w.Body.String())
				}
			})
		}
	}
}

func TestServiceTestMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		check       func(ctx context.Context) (models.ServiceHealth, int)
		wantCode    int
		wantMessage string
	}{
		{
			name: "the service rejects the key",
			check: func(context.Context) (models.ServiceHealth, int) {
				return models.ServiceHealth{Status: "error", Message: "Unauthorized"}, http.StatusUnauthorized
			},
			wantCode:    http.StatusBadGateway,
			wantMessage: "Invalid API key",
		},
		{
			name: "the service fails with its own message",
			check: func(context.Context) (models.ServiceHealth, int) {
				return models.ServiceHealth{Status: "error", Message: "API request failed (Status 403)"}, http.StatusServiceUnavailable
			},
			wantCode:    http.StatusBadGateway,
			wantMessage: "API request failed (Status 403)",
		},
		{
			name: "the service does not answer in time",
			check: func(ctx context.Context) (models.ServiceHealth, int) {
				<-ctx.Done()
				return models.ServiceHealth{Status: "offline", Message: ctx.Err().Error()}, http.StatusOK
			},
			wantCode:    http.StatusGatewayTimeout,
			wantMessage: "The service did not answer within 50ms",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			creator := &mockServiceCreator{
				createServiceFunc: func(string) models.ServiceHealthChecker {
					return &mockServiceHealthChecker{
						checkHealthFunc: func(ctx context.Context, _, _ string) (models.ServiceHealth, int) {
							return tc.check(ctx)
						},
					}
				},
			}
			h := NewHealthHandler(&testing_mocks.MockDB{}, creator)
			h.timeout = 50 * time.Millisecond

			w, _ := postServiceTest(t, h, "sonarr", "http://127.0.0.1:1")

			if w.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantCode)
			}
			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Message != tc.wantMessage {
				t.Fatalf("message = %q, want %q", body.Message, tc.wantMessage)
			}
		})
	}
}

func TestServiceTestUsesTheStoredKeyOfAnOptionalKeyType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &testing_mocks.MockDB{
		FindServiceByFunc: func(_ context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
			return &models.ServiceConfiguration{InstanceID: params.InstanceID, URL: "http://old.test", APIKey: "user:pass"}, nil
		},
	}
	var gotKey string
	creator := &mockServiceCreator{
		createServiceFunc: func(string) models.ServiceHealthChecker {
			return &mockServiceHealthChecker{
				checkHealthFunc: func(_ context.Context, _, apiKey string) (models.ServiceHealth, int) {
					gotKey = apiKey
					return models.ServiceHealth{Status: "online"}, http.StatusOK
				},
			}
		},
	}
	h := NewHealthHandler(db, creator)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health/traefik-1", strings.NewReader(`{"url":"http://new.test:9000"}`))
	w := httptest.NewRecorder()
	serviceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK || gotKey != "user:pass" {
		t.Fatalf("status = %d, key = %q, want 200 with the stored key", w.Code, gotKey)
	}
}
