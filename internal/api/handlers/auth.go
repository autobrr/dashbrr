// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

type AuthHandler struct {
	config       *types.AuthConfig
	cache        cache.Store
	oauth2Config *oauth2.Config
	httpClient   *http.Client
	userinfoURL  string
	mu           sync.RWMutex
	discoverySF  singleflight.Group
}

const oidcSessionLookupTimeout = 5 * time.Second

// Independent of the OIDC provider token, which is discarded after login.
const sessionTTL = 30 * 24 * time.Hour

func NewAuthHandler(config *types.AuthConfig, store cache.Store) *AuthHandler {
	httpClient := &http.Client{Timeout: 1 * time.Second}

	log.Debug().
		Str("issuer", config.Issuer).
		Msg("initializing auth handler")

	return &AuthHandler{
		config:     config,
		cache:      store,
		httpClient: httpClient,
	}
}

func (h *AuthHandler) loadOIDCSession(ctx context.Context, sessionID string) (types.SessionData, error) {
	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(ctx, sessionKey, &sessionData); err != nil {
		return types.SessionData{}, err
	}

	return sessionData, nil
}

func (h *AuthHandler) getOAuthConfig() *oauth2.Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.oauth2Config
}

func (h *AuthHandler) ensureProviderConfig(ctx context.Context) error {
	h.mu.RLock()
	if h.oauth2Config != nil {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	_, err, _ := h.discoverySF.Do("oidc-provider-config", func() (any, error) {
		h.mu.RLock()
		if h.oauth2Config != nil {
			h.mu.RUnlock()
			return nil, nil
		}
		h.mu.RUnlock()

		endpoints, userinfoURL, discoverErr := getProviderEndpoints(ctx, h.httpClient, h.config.Issuer)
		if discoverErr != nil {
			return nil, discoverErr
		}

		oauth2Config := &oauth2.Config{
			ClientID:     h.config.ClientID,
			ClientSecret: h.config.ClientSecret,
			RedirectURL:  h.config.RedirectURL,
			Endpoint:     endpoints,
			Scopes:       []string{"openid", "profile", "email"},
		}

		h.mu.Lock()
		if h.oauth2Config == nil {
			log.Debug().
				Str("auth_url", endpoints.AuthURL).
				Str("token_url", endpoints.TokenURL).
				Msg("using discovered endpoints")
			h.oauth2Config = oauth2Config
			h.userinfoURL = userinfoURL
		}
		h.mu.Unlock()

		return nil, nil
	})

	return err
}

// getProviderEndpoints fetches provider configuration and returns oauth2.Endpoint
// Examples:
// Simple issuer (Google):
//
//	Input:  https://accounts.google.com
//	Result: https://accounts.google.com/.well-known/openid-configuration
//
// Path-based (e.g. Keycloak realm):
//
//	Input:  https://auth.example.com/realms/myrealm
//	Result: https://auth.example.com/realms/myrealm/.well-known/openid-configuration
func getProviderEndpoints(ctx context.Context, client *http.Client, issuer string) (oauth2.Endpoint, string, error) {
	issuer = strings.TrimRight(issuer, "/")

	// Construct well-known URL according to spec
	var wellKnown string
	if strings.Contains(issuer, "/.well-known/openid-configuration") {
		wellKnown = issuer
	} else {
		wellKnown = issuer + "/.well-known/openid-configuration"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return oauth2.Endpoint{}, "", fmt.Errorf("discovery document returned status %d", resp.StatusCode)
		}

		msg := strings.TrimSpace(string(snippet))
		if msg == "" {
			return oauth2.Endpoint{}, "", fmt.Errorf("discovery document returned status %d", resp.StatusCode)
		}

		return oauth2.Endpoint{}, "", fmt.Errorf("discovery document returned status %d: %s", resp.StatusCode, msg)
	}

	log.Debug().
		Str("issuer", issuer).
		Str("well_known_url", wellKnown).
		Msg("OIDC discovery successful")

	var discovery struct {
		AuthURL     string `json:"authorization_endpoint"`
		TokenURL    string `json:"token_endpoint"`
		UserinfoURL string `json:"userinfo_endpoint"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("parsing discovery document: %w", err)
	}
	if discovery.AuthURL == "" || discovery.TokenURL == "" {
		return oauth2.Endpoint{}, "", fmt.Errorf("discovery document missing required endpoints")
	}

	return oauth2.Endpoint{
		AuthURL:  discovery.AuthURL,
		TokenURL: discovery.TokenURL,
	}, discovery.UserinfoURL, nil
}

func generateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

func extractJWTNonce(rawIDToken string) (string, error) {
	parts := strings.Split(rawIDToken, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid id_token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode id_token payload: %w", err)
	}

	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse id_token payload: %w", err)
	}
	if claims.Nonce == "" {
		return "", fmt.Errorf("id_token missing nonce")
	}
	return claims.Nonce, nil
}

func buildLogoutURL(issuer string, clientID string, frontendURL string) string {
	logoutBase := strings.TrimRight(issuer, "/") + "/v2/logout"
	logoutURL, err := url.Parse(logoutBase)
	if err != nil {
		return fmt.Sprintf("%s/v2/logout?client_id=%s&returnTo=%s", strings.TrimRight(issuer, "/"), clientID, frontendURL)
	}

	query := logoutURL.Query()
	query.Set("client_id", clientID)
	query.Set("returnTo", frontendURL)
	logoutURL.RawQuery = query.Encode()

	return logoutURL.String()
}

type oidcStateData struct {
	Timestamp   int64  `json:"timestamp"`
	FrontendURL string `json:"frontendUrl"`
	Nonce       string `json:"nonce"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	// Create context with timeout for login flow
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	log.Info().Msg("initiating login flow")

	frontendUrl := c.Query("frontendUrl")
	if frontendUrl == "" {
		log.Error().Msg("no frontend URL provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Frontend URL is required"})
		return
	}

	if err := h.ensureProviderConfig(ctx); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("OIDC discovery canceled during login")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).
			Msg("OIDC discovery failed. Please ensure your provider supports OpenID Connect discovery as specified in https://openid.net/specs/openid-connect-discovery-1_0.html")
		c.JSON(http.StatusBadGateway, gin.H{"error": "OIDC provider discovery failed"})
		return
	}

	state, err := generateSecureRandomString(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	nonce, err := generateSecureRandomString(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate nonce")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	stateKey := fmt.Sprintf("oidc:state:%s", state)

	stateData := oidcStateData{
		Timestamp:   time.Now().Unix(),
		FrontendURL: frontendUrl,
		Nonce:       nonce,
	}

	if err := h.cache.Set(ctx, stateKey, stateData, 5*time.Minute); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while storing state")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).Msg("failed to store state in cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	authURL := h.getOAuthConfig().AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("response_type", "code"),
	)

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	// Create context with timeout for callback handling
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		log.Error().Msg("no code in callback")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=no_code")
		return
	}

	if err := h.ensureProviderConfig(ctx); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("OIDC discovery canceled during callback")
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=timeout")
			return
		}
		log.Error().Err(err).Msg("OIDC discovery failed during callback")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=oidc_discovery_failed")
		return
	}

	stateKey := fmt.Sprintf("oidc:state:%s", state)
	var stateData oidcStateData
	if err := h.cache.Get(ctx, stateKey, &stateData); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while retrieving state")
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=timeout")
			return
		}
		if err == cache.ErrKeyNotFound {
			log.Debug().Msg("state not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get state from cache")
		}
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=invalid_state")
		return
	}

	frontendUrl := stateData.FrontendURL
	expectedNonce := stateData.Nonce
	if frontendUrl == "" || expectedNonce == "" {
		log.Error().Msg("invalid state data")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=invalid_state")
		return
	}

	if err := h.cache.Delete(ctx, stateKey); err != nil {
		if err != cache.ErrKeyNotFound {
			log.Error().Err(err).Msg("failed to delete state from cache")
		}
	}

	// Exchange code for token using context
	token, err := h.getOAuthConfig().Exchange(ctx, code)
	if err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled during token exchange")
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=timeout", frontendUrl))
			return
		}
		log.Error().Err(err).Msg("code exchange failed")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=exchange_failed", frontendUrl))
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		log.Error().Msg("no id_token in token response")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=no_id_token", frontendUrl))
		return
	}

	gotNonce, err := extractJWTNonce(rawIDToken)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract nonce from id_token")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=invalid_nonce", frontendUrl))
		return
	}
	if gotNonce != expectedNonce {
		log.Error().Msg("id_token nonce mismatch")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=invalid_nonce", frontendUrl))
		return
	}

	sessionData := types.SessionData{
		ExpiresAt: time.Now().Add(sessionTTL),
		AuthType:  "oidc",
	}

	// Use a server-generated session ID. Provider access tokens can rotate and should
	// not be used as stable session identifiers.
	sessionID, err := generateSecureRandomString(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate session id")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=session_failed", frontendUrl))
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	if err := h.cache.Set(ctx, sessionKey, sessionData, sessionTTL); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while storing session")
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=timeout", frontendUrl))
			return
		}
		log.Error().Err(err).Msg("failed to store session in cache")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=session_failed", frontendUrl))
		return
	}

	var isSecure = c.GetHeader("X-Forwarded-Proto") == "https"

	c.SetCookie(
		middleware.SessionCookieName,
		sessionID,
		int(sessionTTL.Seconds()),
		"/",
		"",
		isSecure,
		true,
	)

	// Cookie carries the session; avoid leaking tokens in URLs.
	c.Redirect(http.StatusTemporaryRedirect, frontendUrl)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Create context with timeout for logout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	frontendUrl := c.Query("frontendUrl")
	if frontendUrl == "" {
		log.Error().Msg("no frontend URL provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Frontend URL is required"})
		return
	}

	sessionID, err := getSessionToken(c)
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusOK, gin.H{"message": "Already logged out"})
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	if err := h.cache.Delete(ctx, sessionKey); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while deleting session")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		if err != cache.ErrKeyNotFound {
			log.Error().Err(err).Msg("failed to delete session from cache")
		}
	}

	var isSecure = c.GetHeader("X-Forwarded-Proto") == "https"

	c.SetCookie(
		middleware.SessionCookieName,
		"",
		-1,
		"/",
		"",
		isSecure,
		true,
	)

	logoutURL := buildLogoutURL(h.config.Issuer, h.config.ClientID, frontendUrl)
	c.Redirect(http.StatusTemporaryRedirect, logoutURL)
}

func (h *AuthHandler) VerifyToken(c *gin.Context) {
	sessionID, err := getSessionToken(c)
	if err != nil {
		log.Trace().Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Create context with timeout for token verification
	ctx, cancel := context.WithTimeout(c.Request.Context(), oidcSessionLookupTimeout)
	defer cancel()

	if _, err := h.loadOIDCSession(ctx, sessionID); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while verifying token")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		if err == cache.ErrKeyNotFound {
			log.Trace().Msg("session not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get session from cache")
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token is valid",
	})
}

func (h *AuthHandler) UserInfo(c *gin.Context) {
	sessionID, err := getSessionToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), oidcSessionLookupTimeout)
	defer cancel()

	sessionData, err := h.loadOIDCSession(ctx, sessionID)
	if err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while loading user info session")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		if err == cache.ErrKeyNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		}
		log.Error().Err(err).Msg("failed to get session from cache")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	// Just return the basic session info we already have
	c.JSON(http.StatusOK, gin.H{
		"user_id":   sessionData.UserID,
		"auth_type": sessionData.AuthType,
	})
}
