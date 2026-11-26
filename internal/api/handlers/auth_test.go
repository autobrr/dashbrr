// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/dashbrr/internal/api/session"
	"github.com/autobrr/dashbrr/internal/config"
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

	handler := NewAuthHandler(config, "", mockStore, session.New(mockStore, "/"))

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
			"userinfo_endpoint": "%s/userinfo",
			"jwks_uri": "%s/jwks"
		}`, serverURL, serverURL, serverURL, serverURL, serverURL)
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

	handler := NewAuthHandler(config, "", mockStore, session.New(mockStore, "/"))
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

	handler := NewAuthHandler(config, "", mockStore, session.New(mockStore, "/"))
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
			"userinfo_endpoint": "%s/userinfo",
			"jwks_uri": "%s/jwks"
		}`, serverURL, serverURL, serverURL, serverURL, serverURL)
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
	store := new(MockStore)
	handler := NewAuthHandler(config, "", store, session.New(store, "/"))

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

type stubProvider struct {
	t             *testing.T
	server        *httptest.Server
	prefix        string
	issuer        string
	omit          string
	nonce         string
	tokenResponse func(w http.ResponseWriter, p *stubProvider)
}

var (
	providerKey = sync.OnceValue(mustRSAKey)
	otherKey    = sync.OnceValue(mustRSAKey)
)

func mustRSAKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

func newStubProvider(t *testing.T) *stubProvider {
	return newStubProviderAt(t, "", "")
}

func newStubProviderAt(t *testing.T, prefix, issuerSuffix string) *stubProvider {
	t.Helper()
	p := &stubProvider{
		t:      t,
		prefix: prefix,
		tokenResponse: func(w http.ResponseWriter, p *stubProvider) {
			writeToken(w, p.idToken(providerKey(), nil))
		},
	}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ok := strings.CutPrefix(r.URL.Path, p.prefix)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		base := p.server.URL + p.prefix
		switch path {
		case "/.well-known/openid-configuration":
			discovery := map[string]any{
				"issuer":                                p.issuer,
				"authorization_endpoint":                base + "/authorize",
				"token_endpoint":                        base + "/token",
				"jwks_uri":                              base + "/jwks",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			}
			delete(discovery, p.omit)
			w.Header().Set("Content-Type", "application/json")
			_ = jsonv2.MarshalWrite(w, discovery)
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_ = jsonv2.MarshalWrite(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
				{Key: &providerKey().PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"},
			}})
		case "/token":
			p.tokenResponse(w, p)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	p.issuer = p.server.URL + prefix + issuerSuffix
	t.Cleanup(p.server.Close)
	return p
}

func (p *stubProvider) idToken(key *rsa.PrivateKey, edit func(claims map[string]any)) string {
	now := time.Now()
	claims := map[string]any{
		"iss":   p.issuer,
		"sub":   "user-1",
		"aud":   "test-client-id",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"nonce": p.nonce,
	}
	if edit != nil {
		edit(claims)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "test-key"}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(p.t, err)
	payload, err := jsonv2.Marshal(claims)
	require.NoError(p.t, err)
	signed, err := signer.Sign(payload)
	require.NoError(p.t, err)
	token, err := signed.CompactSerialize()
	require.NoError(p.t, err)
	return token
}

func writeToken(w http.ResponseWriter, idToken string) {
	w.Header().Set("Content-Type", "application/json")
	body := map[string]string{"access_token": "access", "token_type": "Bearer"}
	if idToken != "" {
		body["id_token"] = idToken
	}
	_ = json.NewEncoder(w).Encode(body)
}

func newOIDCTestHandler(t *testing.T, issuer string, base config.BasePath) *AuthHandler {
	t.Helper()
	store := newBuiltinMemoryStore(t)
	return NewAuthHandler(&types.AuthConfig{
		Issuer:       issuer,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "https://dashbrr.example.test:8443/api/auth/callback",
	}, base, store, session.New(store, base.Path("/")))
}

func newOIDCTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/oidc/login", h.Login)
	r.GET("/api/auth/callback", h.Callback)
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
	assert.Equal(t, p.server.URL+p.prefix+"/authorize", authURL.Scheme+"://"+authURL.Host+authURL.Path)
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
			r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL, ""))

			state := login(t, r, p, target)
			w := serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), "", nil)

			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, "/", w.Header().Get("Location"))
		})
	}
}

func TestOIDCLoginWithIssuerForms(t *testing.T) {
	tests := []struct {
		name             string
		prefix           string
		advertisedSuffix string
		configured       func(p *stubProvider) string
		wantLocation     string
	}{
		{
			name:         "exact issuer",
			configured:   func(p *stubProvider) string { return p.server.URL },
			wantLocation: "/",
		},
		{
			name:         "issuer with a path",
			prefix:       "/realms/myrealm",
			configured:   func(p *stubProvider) string { return p.server.URL + "/realms/myrealm" },
			wantLocation: "/",
		},
		{
			name:         "configured with a trailing slash the provider does not advertise",
			prefix:       "/realms/myrealm",
			configured:   func(p *stubProvider) string { return p.server.URL + "/realms/myrealm/" },
			wantLocation: "/",
		},
		{
			name:             "provider advertises a trailing slash the configuration lacks",
			prefix:           "/application/o/dashbrr",
			advertisedSuffix: "/",
			configured:       func(p *stubProvider) string { return p.server.URL + "/application/o/dashbrr" },
			wantLocation:     "/",
		},
		{
			name:         "configured as the discovery document URL",
			prefix:       "/realms/myrealm",
			configured:   func(p *stubProvider) string { return p.server.URL + "/realms/myrealm/.well-known/openid-configuration" },
			wantLocation: "/",
		},
		{
			name:   "configured as the discovery document URL with a trailing slash",
			prefix: "/realms/myrealm",
			configured: func(p *stubProvider) string {
				return p.server.URL + "/realms/myrealm/.well-known/openid-configuration/"
			},
			wantLocation: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newStubProviderAt(t, tt.prefix, tt.advertisedSuffix)
			r := newOIDCTestRouter(newOIDCTestHandler(t, tt.configured(p), ""))

			state := login(t, r, p, "/api/auth/oidc/login")
			w := serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), "", nil)

			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, tt.wantLocation, w.Header().Get("Location"))
		})
	}
}

func TestOIDCLoginRejectsProviderWithOtherIssuer(t *testing.T) {
	p := newStubProvider(t)
	p.issuer = "https://other-issuer.example.test"
	r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL, ""))

	w := serve(t, r, http.MethodGet, "/api/auth/oidc/login", "", nil)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestOIDCLoginRejectsDiscoveryWithoutEndpoints(t *testing.T) {
	for _, missing := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri"} {
		t.Run(missing, func(t *testing.T) {
			p := newStubProvider(t)
			p.omit = missing
			r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL, ""))

			w := serve(t, r, http.MethodGet, "/api/auth/oidc/login", "", nil)
			assert.Equal(t, http.StatusBadGateway, w.Code)
		})
	}
}

func TestOIDCCallbackErrorsRedirectToRelativeLogin(t *testing.T) {
	signed := func(key func() *rsa.PrivateKey, edit func(claims map[string]any)) func(http.ResponseWriter, *stubProvider) {
		return func(w http.ResponseWriter, p *stubProvider) { writeToken(w, p.idToken(key(), edit)) }
	}

	tests := []struct {
		name          string
		tokenResponse func(w http.ResponseWriter, p *stubProvider)
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
			tokenResponse: func(w http.ResponseWriter, _ *stubProvider) { w.WriteHeader(http.StatusInternalServerError) },
			want:          "/login?error=exchange_failed",
		},
		{
			name:          "no id_token",
			tokenResponse: func(w http.ResponseWriter, _ *stubProvider) { writeToken(w, "") },
			want:          "/login?error=no_id_token",
		},
		{
			name:          "malformed id_token",
			tokenResponse: func(w http.ResponseWriter, _ *stubProvider) { writeToken(w, "garbage") },
			want:          "/login?error=invalid_id_token",
		},
		{
			name:          "unsigned id_token",
			tokenResponse: func(w http.ResponseWriter, p *stubProvider) { writeToken(w, unsignedToken(p)) },
			want:          "/login?error=invalid_id_token",
		},
		{
			name:          "bad signature",
			tokenResponse: signed(otherKey, nil),
			want:          "/login?error=invalid_id_token",
		},
		{
			name:          "wrong audience",
			tokenResponse: signed(providerKey, func(c map[string]any) { c["aud"] = "other-client" }),
			want:          "/login?error=invalid_id_token",
		},
		{
			name:          "wrong issuer",
			tokenResponse: signed(providerKey, func(c map[string]any) { c["iss"] = "https://other-issuer.example.test" }),
			want:          "/login?error=invalid_id_token",
		},
		{
			name: "expired",
			tokenResponse: signed(providerKey, func(c map[string]any) {
				c["iat"] = time.Now().Add(-2 * time.Hour).Unix()
				c["exp"] = time.Now().Add(-time.Hour).Unix()
			}),
			want: "/login?error=invalid_id_token",
		},
		{
			name:          "nonce mismatch",
			tokenResponse: signed(providerKey, func(c map[string]any) { c["nonce"] = "other" }),
			want:          "/login?error=invalid_nonce",
		},
		{
			name:          "missing nonce",
			tokenResponse: signed(providerKey, func(c map[string]any) { delete(c, "nonce") }),
			want:          "/login?error=invalid_nonce",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newStubProvider(t)
			if tt.tokenResponse != nil {
				p.tokenResponse = tt.tokenResponse
			}
			r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL, ""))

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

func unsignedToken(p *stubProvider) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := jsonv2.Marshal(map[string]any{
		"iss": p.issuer, "sub": "user-1", "aud": "test-client-id",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "nonce": p.nonce,
	})
	require.NoError(p.t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func TestOIDCCallbackUnderBasePath(t *testing.T) {
	p := newStubProvider(t)
	r := newOIDCTestRouter(newOIDCTestHandler(t, p.server.URL, "/dashbrr"))

	w := serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state=unknown", "", nil)
	assert.Equal(t, "/dashbrr/login?error=invalid_state", w.Header().Get("Location"))

	state := login(t, r, p, "/api/auth/oidc/login")
	w = serve(t, r, http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), "", nil)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "/dashbrr/", w.Header().Get("Location"))

	var cookiePath string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == session.CookieName {
			cookiePath = cookie.Path
		}
	}
	assert.Equal(t, "/dashbrr/", cookiePath)
}
