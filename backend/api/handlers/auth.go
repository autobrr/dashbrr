package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/autobrr/dashbrr/backend/services/cache"
	"github.com/autobrr/dashbrr/backend/types"
)

type AuthHandler struct {
	config       *types.AuthConfig
	cache        cache.CacheInterface
	oauth2Config *oauth2.Config
}

func NewAuthHandler(config *types.AuthConfig, cache cache.CacheInterface) *AuthHandler {
	// Ensure issuer URL doesn't have trailing slash
	issuer := strings.TrimRight(config.Issuer, "/")

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/authorize", issuer),
			TokenURL: fmt.Sprintf("%s/oauth/token", issuer),
		},
		Scopes: []string{"openid", "profile", "email"},
	}

	return &AuthHandler{
		config:       config,
		cache:        cache,
		oauth2Config: oauth2Config,
	}
}

// generateSecureRandomString generates a cryptographically secure random string
func generateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// Login initiates the OIDC authentication flow
func (h *AuthHandler) Login(c *gin.Context) {
	log.Info().Msg("initiating login flow")

	// Get frontend URL from query parameter
	frontendUrl := c.Query("frontendUrl")
	if frontendUrl == "" {
		log.Error().Msg("no frontend URL provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Frontend URL is required"})
		return
	}

	// Generate state parameter
	state, err := generateSecureRandomString(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Generate nonce parameter
	nonce, err := generateSecureRandomString(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate nonce")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Store state and nonce in Redis
	stateKey := fmt.Sprintf("oidc:state:%s", state)
	nonceKey := fmt.Sprintf("oidc:nonce:%s", nonce)

	// Store both state and frontend URL
	stateData := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"frontendUrl": frontendUrl,
	}

	if err := h.cache.Set(c, stateKey, stateData, 5*time.Minute); err != nil {
		log.Error().Err(err).Msg("failed to store state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if err := h.cache.Set(c, nonceKey, time.Now().Unix(), 5*time.Minute); err != nil {
		log.Error().Err(err).Msg("failed to store nonce")
		_ = h.cache.Delete(c, stateKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Build Auth0 authorize URL with additional parameters
	authURL := h.oauth2Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("response_type", "code"),
	)

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles the OIDC provider callback
func (h *AuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		log.Error().Msg("no code in callback")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=no_code")
		return
	}

	// Verify state and get frontend URL
	stateKey := fmt.Sprintf("oidc:state:%s", state)
	var stateData map[string]interface{}
	if err := h.cache.Get(c, stateKey, &stateData); err != nil {
		log.Error().Err(err).Msg("invalid or expired state")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=invalid_state")
		return
	}

	frontendUrl, ok := stateData["frontendUrl"].(string)
	if !ok {
		log.Error().Msg("no frontend URL in state data")
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=invalid_state")
		return
	}

	// Clean up used state
	if err := h.cache.Delete(c, stateKey); err != nil {
		log.Error().Err(err).Msg("failed to delete state")
	}

	// Exchange code for token
	token, err := h.oauth2Config.Exchange(c, code)
	if err != nil {
		log.Error().Err(err).Msg("code exchange failed")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=exchange_failed", frontendUrl))
		return
	}

	// Extract ID Token claims
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		log.Error().Msg("no id_token in token response")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=no_id_token", frontendUrl))
		return
	}

	// Store session
	sessionData := types.SessionData{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		IDToken:      rawIDToken,
		ExpiresAt:    token.Expiry,
		AuthType:     "oidc",
	}

	sessionKey := fmt.Sprintf("oidc:session:%s", token.AccessToken)
	if err := h.cache.Set(c, sessionKey, sessionData, time.Until(token.Expiry)); err != nil {
		log.Error().Err(err).Msg("failed to store session")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=session_failed", frontendUrl))
		return
	}

	// Set secure cookie with session ID
	c.SetCookie(
		"session",
		token.AccessToken,
		int(time.Until(token.Expiry).Seconds()),
		"/",
		"",
		true, // Secure
		true, // HttpOnly
	)

	// Redirect to frontend with tokens
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?access_token=%s&id_token=%s",
		frontendUrl,
		token.AccessToken,
		rawIDToken,
	))
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get frontend URL from query parameter
	frontendUrl := c.Query("frontendUrl")
	if frontendUrl == "" {
		log.Error().Msg("no frontend URL provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Frontend URL is required"})
		return
	}

	// Get session cookie
	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusOK, gin.H{"message": "Already logged out"})
		return
	}

	// Delete session from Redis
	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	if err := h.cache.Delete(c, sessionKey); err != nil {
		log.Error().Err(err).Msg("failed to delete session")
	}

	// Clear session cookie
	c.SetCookie(
		"session",
		"",
		-1,
		"/",
		"",
		true,
		true,
	)

	// Redirect to Auth0 logout
	logoutURL := fmt.Sprintf("%s/v2/logout?client_id=%s&returnTo=%s",
		strings.TrimRight(h.config.Issuer, "/"),
		h.config.ClientID,
		frontendUrl,
	)
	c.Redirect(http.StatusTemporaryRedirect, logoutURL)
}

// VerifyToken verifies a JWT token
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	// Get session cookie
	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Verify session exists in Redis
	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(c, sessionKey, &sessionData); err != nil {
		log.Error().Err(err).Msg("session not found or expired")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token is valid",
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Get session cookie
	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Get session data from Redis
	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(c, sessionKey, &sessionData); err != nil {
		log.Error().Err(err).Msg("failed to get session data")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		return
	}

	// Create token source for refresh
	token := &oauth2.Token{
		RefreshToken: sessionData.RefreshToken,
		Expiry:       sessionData.ExpiresAt,
	}

	// Refresh the token
	newToken, err := h.oauth2Config.TokenSource(c, token).Token()
	if err != nil {
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
	if err := h.cache.Set(c, sessionKey, sessionData, time.Until(newToken.Expiry)); err != nil {
		log.Error().Err(err).Msg("failed to update session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	// Update session cookie
	c.SetCookie(
		"session",
		newToken.AccessToken,
		int(time.Until(newToken.Expiry).Seconds()),
		"/",
		"",
		true,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newToken.AccessToken,
		"token_type":    newToken.TokenType,
		"expires_in":    int(time.Until(newToken.Expiry).Seconds()),
		"refresh_token": newToken.RefreshToken,
	})
}

// UserInfo returns the current user's information
func (h *AuthHandler) UserInfo(c *gin.Context) {
	// Get session cookie
	sessionID, err := c.Cookie("session")
	if err != nil {
		log.Error().Err(err).Msg("no session cookie found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Get session data from Redis
	sessionKey := fmt.Sprintf("oidc:session:%s", sessionID)
	var sessionData types.SessionData
	if err := h.cache.Get(c, sessionKey, &sessionData); err != nil {
		log.Error().Err(err).Msg("failed to get session data")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		return
	}

	// Make request to userinfo endpoint
	userinfoURL := fmt.Sprintf("%s/userinfo", strings.TrimRight(h.config.Issuer, "/"))
	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create userinfo request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sessionData.AccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("userinfo request failed")
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
