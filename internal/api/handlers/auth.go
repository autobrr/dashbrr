// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
}

func NewAuthHandler(config *types.AuthConfig, store cache.Store) *AuthHandler {
	// Create HTTP client with timeout
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Create context with timeout for provider configuration
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Debug().
		Str("issuer", config.Issuer).
		Msg("initializing auth handler")

	// Try discovery first
	provider, err := getProviderEndpoints(ctx, httpClient, config.Issuer)
	if err != nil {
		// Fall back to default endpoints
		provider = oauth2.Endpoint{
			AuthURL:  config.Issuer + "/authorize",
			TokenURL: config.Issuer + "/oauth/token",
		}
		log.Debug().
			Err(err).
			Str("auth_url", provider.AuthURL).
			Str("token_url", provider.TokenURL).
			Msg("discovery failed, using default endpoints")
	} else {
		log.Debug().
			Str("auth_url", provider.AuthURL).
			Str("token_url", provider.TokenURL).
			Msg("using discovered endpoints")
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider,
		Scopes:       []string{"openid", "profile", "email"},
	}

	return &AuthHandler{
		config:       config,
		cache:        store,
		oauth2Config: oauth2Config,
		httpClient:   httpClient,
	}
}

// getProviderEndpoints fetches provider configuration and returns oauth2.Endpoint
// Examples:
// Simple issuer (Google):
//   Input:  https://accounts.google.com
//   Result: https://accounts.google.com/.well-known/openid-configuration
//
// Path-based (e.g. Keycloak realm):
//   Input:  https://auth.example.com/realms/myrealm
//   Result: https://auth.example.com/realms/myrealm/.well-known/openid-configuration
func getProviderEndpoints(ctx context.Context, client *http.Client, issuer string) (oauth2.Endpoint, error) {

	issuer = strings.TrimRight(issuer, "/")

	// Construct well-known URL according to spec
	var wellKnown string
	u, err := url.Parse(issuer)
	if err != nil {
		return oauth2.Endpoint{}, fmt.Errorf("invalid issuer URL: %w", err)
	}

	if u.Path == "" || u.Path == "/" {
		// No path component: /.well-known/openid-configuration
		wellKnown = issuer + "/.well-known/openid-configuration"
	} else {
		// Has path component: /path/.well-known/openid-configuration
		base := strings.TrimRight(u.Path, "/")
		u.Path = base + "/.well-known/openid-configuration"
		wellKnown = u.String()
	}

	log.Debug().
		Str("issuer", issuer).
		Str("well_known_url", wellKnown).
		Msg("attempting to fetch provider configuration")

	req, err := http.NewRequestWithContext(ctx, "GET", wellKnown, nil)
	if err != nil {
		return oauth2.Endpoint{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return oauth2.Endpoint{}, fmt.Errorf("failed to fetch provider config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return oauth2.Endpoint{}, fmt.Errorf("discovery request failed: %d, body: %s", resp.StatusCode, string(body))
	}

	var config struct {
		AuthURL  string `json:"authorization_endpoint"`
		TokenURL string `json:"token_endpoint"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return oauth2.Endpoint{}, fmt.Errorf("failed to decode provider config: %w", err)
	}

	if config.AuthURL == "" || config.TokenURL == "" {
		return oauth2.Endpoint{}, fmt.Errorf("provider config missing required endpoints: auth_url=%s, token_url=%s", config.AuthURL, config.TokenURL)
	}

	return oauth2.Endpoint{
		AuthURL:  config.AuthURL,
		TokenURL: config.TokenURL,
	}, nil
}

func generateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
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
	nonceKey := fmt.Sprintf("oidc:nonce:%s", nonce)

	stateData := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"frontendUrl": frontendUrl,
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

	if err := h.cache.Set(ctx, nonceKey, time.Now().Unix(), 5*time.Minute); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while storing nonce")
			_ = h.cache.Delete(ctx, stateKey)
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).Msg("failed to store nonce in cache")
		_ = h.cache.Delete(ctx, stateKey)
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

	sessionData := types.SessionData{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		IDToken:      rawIDToken,
		ExpiresAt:    token.Expiry,
		AuthType:     "oidc",
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", token.AccessToken)
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
		token.AccessToken,
		int(time.Until(token.Expiry).Seconds()),
		"/",
		"",
		isSecure,
		true,
	)

	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?access_token=%s&id_token=%s",
		frontendUrl,
		token.AccessToken,
		rawIDToken,
	))
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

	sessionID, err := c.Cookie("session")
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
	// Create context with timeout for token verification
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Trace().Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

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
	// Create context with timeout for token refresh
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(ctx, sessionKey, &sessionData); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while getting session")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		if err == cache.ErrKeyNotFound {
			log.Debug().Msg("session not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get session from cache")
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		return
	}

	token := &oauth2.Token{
		RefreshToken: sessionData.RefreshToken,
		Expiry:       sessionData.ExpiresAt,
	}

	// Create token source with context
	tokenSource := h.oauth2Config.TokenSource(ctx, token)

	// Refresh the token
	newToken, err := tokenSource.Token()
	if err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled during token refresh")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).Msg("token refresh failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	// Update session with new token data
	sessionData.AccessToken = newToken.AccessToken
	sessionData.RefreshToken = newToken.RefreshToken
	sessionData.ExpiresAt = newToken.Expiry
	if rawIDToken, ok := newToken.Extra("id_token").(string); ok {
		sessionData.IDToken = rawIDToken
	}

	// Store updated session
	if err := h.cache.Set(ctx, sessionKey, sessionData, time.Until(newToken.Expiry)); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while updating session")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).Msg("failed to update session in cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	var isSecure = c.GetHeader("X-Forwarded-Proto") == "https"

	c.SetCookie(
		"session",
		newToken.AccessToken,
		int(time.Until(newToken.Expiry).Seconds()),
		"/",
		"",
		isSecure,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newToken.AccessToken,
		"token_type":    newToken.TokenType,
		"expires_in":    int(time.Until(newToken.Expiry).Seconds()),
		"refresh_token": newToken.RefreshToken,
	})
}

func (h *AuthHandler) UserInfo(c *gin.Context) {
	// Create context with timeout for userinfo request
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(ctx, sessionKey, &sessionData); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while getting session")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		if err == cache.ErrKeyNotFound {
			log.Debug().Msg("session not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get session from cache")
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		return
	}

	userinfoURL := fmt.Sprintf("%s/userinfo", strings.TrimRight(h.config.Issuer, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", userinfoURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create userinfo request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sessionData.AccessToken))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled during userinfo request")
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Operation timed out"})
			return
		}
		log.Error().Err(err).Msg("userinfo request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	if resp == nil {
		log.Error().Msg("received nil response from userinfo endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Msg("userinfo request returned non-200 status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		log.Error().Err(err).Msg("failed to decode userinfo response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user info"})
		return
	}

	c.JSON(http.StatusOK, userInfo)
}
