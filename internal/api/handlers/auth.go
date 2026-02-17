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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

type AuthHandler struct {
	config       *types.AuthConfig
	cache        cache.Store
	oauth2Config *oauth2.Config
	httpClient   *http.Client
	userinfoURL  string
}

func NewAuthHandler(config *types.AuthConfig, store cache.Store) *AuthHandler {
	httpClient := &http.Client{Timeout: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	log.Debug().
		Str("issuer", config.Issuer).
		Msg("initializing auth handler")

	// Get provider endpoints through OIDC discovery
	endpoints, userinfoURL, err := getProviderEndpoints(ctx, httpClient, config.Issuer)
	if err != nil {
		log.Error().Err(err).
			Msg("OIDC discovery failed. Please ensure your provider supports OpenID Connect discovery as specified in https://openid.net/specs/openid-connect-discovery-1_0.html")
		return nil
	}

	log.Debug().
		Str("auth_url", endpoints.AuthURL).
		Str("token_url", endpoints.TokenURL).
		Msg("using discovered endpoints")

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     endpoints,
		Scopes:       []string{"openid", "profile", "email"},
	}

	return &AuthHandler{
		config:       config,
		cache:        store,
		oauth2Config: oauth2Config,
		httpClient:   httpClient,
		userinfoURL:  userinfoURL,
	}
}

type providerConfig struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
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

	req, err := http.NewRequestWithContext(ctx, "GET", wellKnown, nil)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("reading discovery document: %w", err)
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

	if err := json.Unmarshal(body, &discovery); err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("parsing discovery document: %w", err)
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

	stateData := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"frontendUrl": frontendUrl,
		"nonce":       nonce,
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

	authURL := h.oauth2Config.AuthCodeURL(
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

	stateKey := fmt.Sprintf("oidc:state:%s", state)
	var stateData map[string]interface{}
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

	frontendUrl, ok := stateData["frontendUrl"].(string)
	if !ok {
		log.Error().Msg("no frontend URL in state data")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=invalid_state")
		return
	}
	expectedNonce, ok := stateData["nonce"].(string)
	if !ok || expectedNonce == "" {
		log.Error().Msg("no nonce in state data")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=invalid_state", frontendUrl))
		return
	}

	if err := h.cache.Delete(ctx, stateKey); err != nil {
		if err != cache.ErrKeyNotFound {
			log.Error().Err(err).Msg("failed to delete state from cache")
		}
	}

	// Exchange code for token using context
	token, err := h.oauth2Config.Exchange(ctx, code)
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
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		IDToken:      rawIDToken,
		ExpiresAt:    token.Expiry,
		AuthType:     "oidc",
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
	if err := h.cache.Set(ctx, sessionKey, sessionData, time.Until(token.Expiry)); err != nil {
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
		"session",
		sessionID,
		int(time.Until(token.Expiry).Seconds()),
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
		"session",
		"",
		-1,
		"/",
		"",
		isSecure,
		true,
	)

	logoutURL := fmt.Sprintf("%s/v2/logout?client_id=%s&returnTo=%s",
		strings.TrimRight(h.config.Issuer, "/"),
		h.config.ClientID,
		frontendUrl,
	)
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(ctx, sessionKey, &sessionData); err != nil {
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

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	sessionID, err := getSessionToken(c)
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(c.Request.Context(), sessionKey, &sessionData); err != nil {
		if err == cache.ErrKeyNotFound {
			log.Debug().Msg("session not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get session from cache")
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
		return
	}

	token := &oauth2.Token{
		AccessToken:  sessionData.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: sessionData.RefreshToken,
		Expiry:       sessionData.ExpiresAt,
	}

	// Create token source with context
	tokenSource := h.oauth2Config.TokenSource(c.Request.Context(), token)

	// Refresh the token
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Error().Err(err).Msg("token refresh failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	// Update session data with new token
	sessionData.AccessToken = newToken.AccessToken
	// Some providers omit refresh_token on refresh. Preserve the existing token.
	if newToken.RefreshToken != "" {
		sessionData.RefreshToken = newToken.RefreshToken
	}
	sessionData.ExpiresAt = newToken.Expiry
	if rawIDToken, ok := newToken.Extra("id_token").(string); ok {
		sessionData.IDToken = rawIDToken
	}

	// Store updated session
	if err := h.cache.Set(c.Request.Context(), sessionKey, sessionData, time.Until(newToken.Expiry)); err != nil {
		log.Error().Err(err).Msg("failed to update session in cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newToken.AccessToken,
		"token_type":    newToken.TokenType,
		"expires_in":    int(time.Until(newToken.Expiry).Seconds()),
		"refresh_token": newToken.RefreshToken,
	})
}

func (h *AuthHandler) UserInfo(c *gin.Context) {
	sessionID, err := getSessionToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(c.Request.Context(), sessionKey, &sessionData); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	// Just return the basic session info we already have
	c.JSON(http.StatusOK, gin.H{
		"user_id":   sessionData.UserID,
		"auth_type": sessionData.AuthType,
	})
}
