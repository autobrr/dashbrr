// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/api/session"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

type AuthHandler struct {
	config       *types.AuthConfig
	basePath     config.BasePath
	cache        cache.Store
	sessions     *session.Manager
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	httpClient   *http.Client
	mu           sync.RWMutex
	discoverySF  singleflight.Group
}

func NewAuthHandler(authConfig *types.AuthConfig, basePath config.BasePath, store cache.Store, sessions *session.Manager) *AuthHandler {
	httpClient := &http.Client{Timeout: 1 * time.Second}

	log.Debug().
		Str("issuer", authConfig.Issuer).
		Msg("initializing auth handler")

	return &AuthHandler{
		config:     authConfig,
		basePath:   basePath,
		cache:      store,
		sessions:   sessions,
		httpClient: httpClient,
	}
}

func (h *AuthHandler) getOAuthConfig() *oauth2.Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.oauth2Config
}

func (h *AuthHandler) getVerifier() *oidc.IDTokenVerifier {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.verifier
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

		provider, discoverErr := discoverProvider(oidc.ClientContext(ctx, h.httpClient), h.config.Issuer)
		if discoverErr != nil {
			return nil, discoverErr
		}

		oauth2Config := &oauth2.Config{
			ClientID:     h.config.ClientID,
			ClientSecret: h.config.ClientSecret,
			RedirectURL:  h.config.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}

		h.mu.Lock()
		if h.oauth2Config == nil {
			log.Debug().
				Str("auth_url", oauth2Config.Endpoint.AuthURL).
				Str("token_url", oauth2Config.Endpoint.TokenURL).
				Msg("using discovered endpoints")
			h.oauth2Config = oauth2Config
			h.verifier = provider.Verifier(&oidc.Config{ClientID: h.config.ClientID})
		}
		h.mu.Unlock()

		return nil, nil
	})

	return err
}

func discoverProvider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	issuer = strings.TrimSuffix(strings.TrimSuffix(issuer, "/"), "/.well-known/openid-configuration")
	provider, err := oidc.NewProvider(ctx, issuer)
	if mismatch, ok := errors.AsType[*oidc.IssuerMismatchError](err); ok && strings.TrimSuffix(mismatch.Discovered, "/") == issuer {
		provider, err = oidc.NewProvider(ctx, mismatch.Discovered)
	}
	if err != nil {
		return nil, err
	}
	if endpoint := provider.Endpoint(); endpoint.AuthURL == "" || endpoint.TokenURL == "" {
		return nil, errors.New("discovery document missing required endpoints")
	}
	return provider, nil
}

func generateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

type oidcStateData struct {
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	// Create context with timeout for login flow
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	log.Info().Msg("initiating login flow")

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
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
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
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=no_code"))
		return
	}

	if err := h.ensureProviderConfig(ctx); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("OIDC discovery canceled during callback")
			c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=timeout"))
			return
		}
		log.Error().Err(err).Msg("OIDC discovery failed during callback")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=oidc_discovery_failed"))
		return
	}

	stateKey := fmt.Sprintf("oidc:state:%s", state)
	var stateData oidcStateData
	if err := h.cache.Get(ctx, stateKey, &stateData); err != nil {
		if ctx.Err() != nil {
			log.Error().Err(ctx.Err()).Msg("Context canceled while retrieving state")
			c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=timeout"))
			return
		}
		if err == cache.ErrKeyNotFound {
			log.Debug().Msg("state not found or expired")
		} else {
			log.Error().Err(err).Msg("failed to get state from cache")
		}
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=invalid_state"))
		return
	}

	expectedNonce := stateData.Nonce
	if expectedNonce == "" {
		log.Error().Msg("invalid state data")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=invalid_state"))
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
			c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=timeout"))
			return
		}
		log.Error().Err(err).Msg("code exchange failed")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=exchange_failed"))
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		log.Error().Msg("no id_token in token response")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=no_id_token"))
		return
	}

	idToken, err := h.getVerifier().Verify(ctx, rawIDToken)
	if err != nil {
		log.Error().Err(err).Msg("id_token verification failed")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=invalid_id_token"))
		return
	}
	if idToken.Nonce != expectedNonce {
		log.Error().Msg("id_token nonce mismatch")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=invalid_nonce"))
		return
	}

	if _, err := h.sessions.Issue(c, types.SessionData{AuthType: "oidc"}); err != nil {
		log.Error().Err(err).Msg("failed to create session")
		c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/login?error=session_failed"))
		return
	}

	// Cookie carries the session; avoid leaking tokens in URLs.
	c.Redirect(http.StatusTemporaryRedirect, h.basePath.Path("/"))
}
