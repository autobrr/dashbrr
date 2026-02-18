// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/internal/utils"
)

const defaultPlexProduct = "Dashbrr"

type PlexAuthHandler struct {
	plexService plexPINService
}

type plexPINService interface {
	CreateAuthPIN(ctx context.Context, clientIdentifier, product string) (*types.PlexPIN, error)
	CheckAuthPIN(ctx context.Context, pinID int, code, clientIdentifier, product string) (*types.PlexPIN, error)
}

type createPlexPINRequest struct {
	Product    string `json:"product"`
	ForwardURL string `json:"forwardUrl"`
}

type createPlexPINResponse struct {
	PinID            int    `json:"pinId"`
	Code             string `json:"code"`
	ClientIdentifier string `json:"clientIdentifier"`
	AuthURL          string `json:"authUrl"`
	ExpiresIn        int    `json:"expiresIn"`
}

type plexPINStatusResponse struct {
	Authorized bool   `json:"authorized"`
	AuthToken  string `json:"authToken,omitempty"`
	ExpiresIn  int    `json:"expiresIn"`
}

func NewPlexAuthHandler() *PlexAuthHandler {
	return &PlexAuthHandler{
		plexService: plex.NewPlexService().(*plex.PlexService),
	}
}

func (h *PlexAuthHandler) CreatePIN(c *gin.Context) {
	var req createPlexPINRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	product := strings.TrimSpace(req.Product)
	if product == "" {
		product = defaultPlexProduct
	}

	clientToken, err := utils.GenerateSecureToken(24)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate Plex client identifier")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Plex authentication"})
		return
	}
	clientIdentifier := "dashbrr-" + clientToken

	pin, err := h.plexService.CreateAuthPIN(c.Request.Context(), clientIdentifier, product)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Plex auth pin")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to create Plex authentication PIN"})
		return
	}

	authURL := buildPlexAuthURL(clientIdentifier, pin.Code, product, req.ForwardURL)

	c.JSON(http.StatusOK, createPlexPINResponse{
		PinID:            pin.ID,
		Code:             pin.Code,
		ClientIdentifier: clientIdentifier,
		AuthURL:          authURL,
		ExpiresIn:        pin.ExpiresIn,
	})
}

func (h *PlexAuthHandler) GetPIN(c *gin.Context) {
	pinID, err := strconv.Atoi(c.Param("pinId"))
	if err != nil || pinID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pin ID"})
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PIN code is required"})
		return
	}

	clientIdentifier := strings.TrimSpace(c.Query("clientIdentifier"))
	if clientIdentifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client identifier is required"})
		return
	}

	product := strings.TrimSpace(c.Query("product"))
	if product == "" {
		product = defaultPlexProduct
	}

	pin, err := h.plexService.CheckAuthPIN(c.Request.Context(), pinID, code, clientIdentifier, product)
	if err != nil {
		log.Error().Err(err).Int("pin_id", pinID).Msg("Failed to query Plex auth pin")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to check Plex authentication status"})
		return
	}

	authToken := strings.TrimSpace(pin.AuthToken)
	c.JSON(http.StatusOK, plexPINStatusResponse{
		Authorized: authToken != "",
		AuthToken:  authToken,
		ExpiresIn:  pin.ExpiresIn,
	})
}

func buildPlexAuthURL(clientIdentifier, code, product, forwardURL string) string {
	query := url.Values{}
	query.Set("clientID", clientIdentifier)
	query.Set("code", code)
	query.Set("context[device][product]", product)
	query.Set("context[device][version]", "1.0.0")

	trimmedForwardURL := strings.TrimSpace(forwardURL)
	if trimmedForwardURL != "" {
		query.Set("forwardUrl", trimmedForwardURL)
	}

	return "https://app.plex.tv/auth#?" + query.Encode()
}
