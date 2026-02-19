// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

// MockStore is a mock implementation of cache.Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Get(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockStore) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStore) Increment(ctx context.Context, key string, timestamp int64) error {
	args := m.Called(ctx, key, timestamp)
	return args.Error(0)
}

func (m *MockStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	args := m.Called(ctx, key, windowStart)
	return args.Error(0)
}

func (m *MockStore) GetCount(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return 0, args.Error(1)
	}
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

type blockingStore struct{}

func (blockingStore) Get(ctx context.Context, _ string, _ interface{}) error {
	<-ctx.Done()
	return ctx.Err()
}
func (blockingStore) Set(context.Context, string, interface{}, time.Duration) error { return nil }
func (blockingStore) Delete(context.Context, string) error                          { return nil }
func (blockingStore) Close() error                                                  { return nil }
func (blockingStore) Increment(context.Context, string, int64) error                { return nil }
func (blockingStore) CleanAndCount(context.Context, string, int64) error            { return nil }
func (blockingStore) GetCount(context.Context, string) (int64, error)               { return 0, nil }
func (blockingStore) Expire(context.Context, string, time.Duration) error           { return nil }

func TestNewAuthHandler(t *testing.T) {
	config := &types.AuthConfig{
		Issuer:       "https://example.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	mockStore := new(MockStore)

	handler := NewAuthHandler(config, mockStore)

	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.Nil(t, handler.oauth2Config)
}

func TestAuthHandlerEnsureProviderConfig(t *testing.T) {
	var serverURL string

	// Create a test server that responds to OIDC discovery
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("Expected request to /.well-known/openid-configuration, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{
			"issuer": "%s",
			"authorization_endpoint": "%s/authorize",
			"token_endpoint": "%s/oauth/token",
			"userinfo_endpoint": "%s/userinfo"
		}`, serverURL, serverURL, serverURL, serverURL)
		w.Write([]byte(response))
	}))
	defer ts.Close()
	serverURL = ts.URL

	config := &types.AuthConfig{
		Issuer:       serverURL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	mockStore := new(MockStore)

	handler := NewAuthHandler(config, mockStore)
	assert.NotNil(t, handler)
	assert.Nil(t, handler.oauth2Config)

	err := handler.ensureProviderConfig(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, handler.oauth2Config)
	assert.Equal(t, "test-client-id", handler.oauth2Config.ClientID)
	assert.Equal(t, "test-client-secret", handler.oauth2Config.ClientSecret)
	assert.Equal(t, "http://localhost:3000/callback", handler.oauth2Config.RedirectURL)
}

func TestAuthHandlerEnsureProviderConfig_DiscoveryFailed(t *testing.T) {
	var serverURL string

	// Create a test server that returns an error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	serverURL = ts.URL

	config := &types.AuthConfig{
		Issuer:       serverURL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	mockStore := new(MockStore)

	handler := NewAuthHandler(config, mockStore)
	assert.NotNil(t, handler)

	err := handler.ensureProviderConfig(context.Background())
	assert.Error(t, err)
	assert.Nil(t, handler.oauth2Config)
}

func TestAuthHandlerEnsureProviderConfig_ConcurrentDiscoverySingleflight(t *testing.T) {
	var serverURL string
	var hits atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		hits.Add(1)
		time.Sleep(25 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{
			"issuer": "%s",
			"authorization_endpoint": "%s/authorize",
			"token_endpoint": "%s/oauth/token",
			"userinfo_endpoint": "%s/userinfo"
		}`, serverURL, serverURL, serverURL, serverURL)
		w.Write([]byte(response))
	}))
	defer ts.Close()
	serverURL = ts.URL

	config := &types.AuthConfig{
		Issuer:       serverURL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	handler := NewAuthHandler(config, new(MockStore))

	const workers = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- handler.ensureProviderConfig(context.Background())
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	assert.NotNil(t, handler.oauth2Config)
	assert.Equal(t, int32(1), hits.Load())
}

func TestLogin_NoFrontendURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/login", nil)
	c.Request = req

	mockStore := new(MockStore)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		cache: mockStore,
	}

	handler.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockStore.AssertExpectations(t)
}

func TestCallback_NoCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/callback", nil)
	c.Request = req

	mockStore := new(MockStore)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		config: &types.AuthConfig{
			Issuer:       "https://test.auth0.com",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		cache: mockStore,
	}

	handler.Callback(c)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=no_code")
	mockStore.AssertExpectations(t)
}

func TestLogout_NoFrontendURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/logout", nil)
	c.Request = req

	mockStore := new(MockStore)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		config: &types.AuthConfig{
			Issuer:       "https://test.auth0.com",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		cache: mockStore,
	}

	handler.Logout(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockStore.AssertExpectations(t)
}

func TestGetProviderEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		issuer     string
		mockStatus int
		mockBody   string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "google provider",
			issuer:     "https://accounts.google.com",
			mockStatus: http.StatusOK,
			mockBody:   `{"authorization_endpoint":"https://accounts.google.com/o/oauth2/v2/auth","token_endpoint":"https://oauth2.googleapis.com/token","userinfo_endpoint":"https://openidconnect.googleapis.com/v1/userinfo"}`,
			wantPath:   "/.well-known/openid-configuration",
			wantErr:    false,
		},
		{
			name:       "keycloak provider with path",
			issuer:     "https://auth.example.com/realms/myrealm/",
			mockStatus: http.StatusOK,
			mockBody:   `{"authorization_endpoint":"https://auth.example.com/realms/myrealm/protocol/openid-connect/auth","token_endpoint":"https://auth.example.com/realms/myrealm/protocol/openid-connect/token","userinfo_endpoint":"https://auth.example.com/realms/myrealm/protocol/openid-connect/userinfo"}`,
			wantPath:   "/realms/myrealm/.well-known/openid-configuration",
			wantErr:    false,
		},
		{
			name:       "non-200 status with JSON body",
			issuer:     "https://accounts.google.com",
			mockStatus: http.StatusInternalServerError,
			mockBody:   `{"authorization_endpoint":"https://accounts.google.com/o/oauth2/v2/auth","token_endpoint":"https://oauth2.googleapis.com/token","userinfo_endpoint":"https://openidconnect.googleapis.com/v1/userinfo"}`,
			wantPath:   "/.well-known/openid-configuration",
			wantErr:    true,
		},
		{
			name:       "missing required token endpoint",
			issuer:     "https://accounts.google.com",
			mockStatus: http.StatusOK,
			mockBody:   `{"authorization_endpoint":"https://accounts.google.com/o/oauth2/v2/auth","userinfo_endpoint":"https://openidconnect.googleapis.com/v1/userinfo"}`,
			wantPath:   "/.well-known/openid-configuration",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantPath, r.URL.Path, "unexpected request path")
				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockBody))
			}))
			defer ts.Close()

			// Replace the issuer host with our test server
			u, err := url.Parse(tt.issuer)
			assert.NoError(t, err)
			tsURL, err := url.Parse(ts.URL)
			assert.NoError(t, err)
			u.Host = tsURL.Host
			u.Scheme = tsURL.Scheme
			testIssuer := u.String()

			// Test the function
			endpoint, userinfoURL, err := getProviderEndpoints(context.Background(), http.DefaultClient, testIssuer)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Parse mock response to get expected values
			var config struct {
				AuthURL     string `json:"authorization_endpoint"`
				TokenURL    string `json:"token_endpoint"`
				UserinfoURL string `json:"userinfo_endpoint"`
			}
			err = json.Unmarshal([]byte(tt.mockBody), &config)
			assert.NoError(t, err)

			// Verify endpoints match
			assert.Equal(t, config.AuthURL, endpoint.AuthURL)
			assert.Equal(t, config.TokenURL, endpoint.TokenURL)
			assert.Equal(t, config.UserinfoURL, userinfoURL)
		})
	}
}

func TestBuildLogoutURL(t *testing.T) {
	issuer := "https://test.auth0.com/"
	clientID := "test-client-id"
	frontendURL := "http://localhost:3000/login?next=/settings&msg=a b"

	logoutURL := buildLogoutURL(issuer, clientID, frontendURL)

	parsed, err := url.Parse(logoutURL)
	assert.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "test.auth0.com", parsed.Host)
	assert.Equal(t, "/v2/logout", parsed.Path)
	assert.Equal(t, clientID, parsed.Query().Get("client_id"))
	assert.Equal(t, frontendURL, parsed.Query().Get("returnTo"))
}

func TestUserInfo_SessionLookupTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	baseCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/auth/oidc/userinfo", nil).WithContext(baseCtx)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-session"})
	c.Request = req

	handler := &AuthHandler{
		cache: blockingStore{},
	}

	handler.UserInfo(c)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.Contains(t, w.Body.String(), "Operation timed out")
}

func TestUserInfo_SessionExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/auth/oidc/userinfo", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-session"})
	c.Request = req

	mockStore := new(MockStore)
	mockStore.
		On("Get", mock.Anything, "oidc:session:test-session", mock.AnythingOfType("*types.SessionData")).
		Return(cache.ErrKeyNotFound).
		Once()

	handler := &AuthHandler{
		cache: mockStore,
	}

	handler.UserInfo(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Session expired")
	mockStore.AssertExpectations(t)
}
