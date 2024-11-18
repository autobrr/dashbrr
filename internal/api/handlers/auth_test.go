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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/autobrr/dashbrr/internal/types"
)

// MockStore is a mock implementation of cache.Store
type MockStore struct {
	mock.Mock
}

// safeArgs ensures we always return a valid mock.Arguments
func (m *MockStore) safeArgs(args mock.Arguments) mock.Arguments {
	if args == nil {
		return mock.Arguments{errors.New("mock not configured")}
	}
	return args
}

func (m *MockStore) Get(ctx context.Context, key string, value interface{}) error {
	args := m.safeArgs(m.Called(ctx, key, value))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.safeArgs(m.Called(ctx, key, value, expiration))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) Delete(ctx context.Context, key string) error {
	args := m.safeArgs(m.Called(ctx, key))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) Increment(ctx context.Context, key string, timestamp int64) error {
	args := m.safeArgs(m.Called(ctx, key, timestamp))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	args := m.safeArgs(m.Called(ctx, key, windowStart))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) GetCount(ctx context.Context, key string) (int64, error) {
	args := m.safeArgs(m.Called(ctx, key))
	var count int64
	if args.Get(0) != nil {
		if c, ok := args.Get(0).(int64); ok {
			count = c
		}
	}
	var err error
	if args.Get(1) != nil {
		if e, ok := args.Get(1).(error); ok {
			err = e
		}
	}
	return count, err
}

func (m *MockStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.safeArgs(m.Called(ctx, key, expiration))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockStore) Close() error {
	args := m.safeArgs(m.Called())
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func TestNewAuthHandler(t *testing.T) {
	config := &types.AuthConfig{
		Issuer:       "https://test.auth0.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	mockStore := new(MockStore)

	handler := NewAuthHandler(config, mockStore)

	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.NotNil(t, handler.oauth2Config)
	assert.Equal(t, "test-client-id", handler.oauth2Config.ClientID)
	assert.Equal(t, "test-client-secret", handler.oauth2Config.ClientSecret)
	assert.Equal(t, "http://localhost:3000/callback", handler.oauth2Config.RedirectURL)
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
