// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/base64"
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
	"github.com/stretchr/testify/require"

	"github.com/autobrr/dashbrr/internal/types"
)

// MockStore is a mock implementation of cache.Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Get(ctx context.Context, key string, value any) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
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
		wg.Go(func() {
			errs <- handler.ensureProviderConfig(context.Background())
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	assert.NotNil(t, handler.oauth2Config)
	assert.Equal(t, int32(1), hits.Load())
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
	assert.Equal(t, "/login?error=no_code", w.Header().Get("Location"))
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
	returnTo := "http://localhost:3000"

	logoutURL := buildLogoutURL(issuer, clientID, returnTo)

	parsed, err := url.Parse(logoutURL)
	assert.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "test.auth0.com", parsed.Host)
	assert.Equal(t, "/v2/logout", parsed.Path)
	assert.Equal(t, clientID, parsed.Query().Get("client_id"))
	assert.Equal(t, returnTo, parsed.Query().Get("returnTo"))
}

// stubProvider is an OIDC provider that answers discovery and token requests.
// tokenResponse builds the token endpoint reply from the nonce that Login sent.
type stubProvider struct {
	server        *httptest.Server
	nonce         string
	tokenResponse func(w http.ResponseWriter, nonce string)
}

func newStubProvider(t *testing.T) *stubProvider {
	t.Helper()
	p := &stubProvider{
		tokenResponse: func(w http.ResponseWriter, nonce string) {
			writeToken(w, idTokenWithNonce(nonce))
		},
	}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			fmt.Fprintf(w, `{"authorization_endpoint":"%[1]s/authorize","token_endpoint":"%[1]s/token"}`, p.server.URL)
		case "/token":
			p.tokenResponse(w, p.nonce)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(p.server.Close)
	return p
}

func idTokenWithNonce(nonce string) string {
	payload := base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, `{"nonce":%q}`, nonce))
	return "e30." + payload + ".sig"
}

func writeToken(w http.ResponseWriter, idToken string) {
	w.Header().Set("Content-Type", "application/json")
	body := map[string]string{"access_token": "access", "token_type": "Bearer"}
	if idToken != "" {
		body["id_token"] = idToken
	}
	_ = json.NewEncoder(w).Encode(body)
}

func newOIDCTestHandler(t *testing.T, issuer string) *AuthHandler {
	t.Helper()
	return NewAuthHandler(&types.AuthConfig{
		Issuer:       issuer,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "https://dashbrr.example.test:8443/api/auth/callback",
	}, newBuiltinMemoryStore(t))
}

func newOIDCTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/oidc/login", h.Login)
	r.GET("/api/auth/callback", h.Callback)
	r.GET("/api/auth/oidc/logout", h.Logout)
	return r
}

// login runs Login and returns the state from the provider redirect.
// It stores the nonce on the stub provider for the token response.
func login(t *testing.T, r *gin.Engine, p *stubProvider, target string) string {
	t.Helper()
	w := serve(t, r, http.MethodGet, target, "", nil)
	require.Equal(t, http.StatusTemporaryRedirect, w.Code, w.Body.String())

	authURL, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, p.server.URL+"/authorize", authURL.Scheme+"://"+authURL.Host+authURL.Path)
	p.nonce = authURL.Query().Get("nonce")
	return authURL.Query().Get("state")
}

func TestOIDCLoginIgnoresFrontendURL(t *testing.T) {
	for _, target := range []string{
		"/api/auth/oidc/login?frontendUrl=https://attacker.example",
		"/api/auth/oidc/login",
	} {
		t.Run(target, func(t *testing.T) {
			p := newStubProvider(t)
			r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL))

			state := login(t, r, p, target)
			w := serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), "", nil)

			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, "/", w.Header().Get("Location"))
		})
	}
}

func TestOIDCCallbackErrorsRedirectToRelativeLogin(t *testing.T) {
	tests := []struct {
		name          string
		tokenResponse func(w http.ResponseWriter, nonce string)
		skipLogin     bool
		want          string
	}{
		{
			name:      "unknown state",
			skipLogin: true,
			want:      "/login?error=invalid_state",
		},
		{
			name:          "exchange failed",
			tokenResponse: func(w http.ResponseWriter, _ string) { w.WriteHeader(http.StatusInternalServerError) },
			want:          "/login?error=exchange_failed",
		},
		{
			name:          "no id_token",
			tokenResponse: func(w http.ResponseWriter, _ string) { writeToken(w, "") },
			want:          "/login?error=no_id_token",
		},
		{
			name:          "malformed id_token",
			tokenResponse: func(w http.ResponseWriter, _ string) { writeToken(w, "garbage") },
			want:          "/login?error=invalid_nonce",
		},
		{
			name:          "nonce mismatch",
			tokenResponse: func(w http.ResponseWriter, _ string) { writeToken(w, idTokenWithNonce("other")) },
			want:          "/login?error=invalid_nonce",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newStubProvider(t)
			if tt.tokenResponse != nil {
				p.tokenResponse = tt.tokenResponse
			}
			r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL))

			state := "unknown"
			if !tt.skipLogin {
				state = login(t, r, p, "/api/auth/oidc/login?frontendUrl=https://attacker.example")
			}
			w := serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), "", nil)

			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, tt.want, w.Header().Get("Location"))
		})
	}
}

func TestOIDCLogoutReturnsToConfiguredOrigin(t *testing.T) {
	r := newOIDCTestRouter(newOIDCTestHandler(t, "https://provider.example.test"))

	w := serve(t, r, http.MethodGet, "/api/auth/oidc/logout?frontendUrl=https://attacker.example", "", nil)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	logoutURL, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "provider.example.test", logoutURL.Host)
	assert.Equal(t, "https://dashbrr.example.test:8443", logoutURL.Query().Get("returnTo"))
}
