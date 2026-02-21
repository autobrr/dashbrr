// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/types"
)

type stubPlexPINService struct {
	createPIN      *types.PlexPIN
	createErr      error
	createClientID string
	createProduct  string
	checkPIN       *types.PlexPIN
	checkErr       error
	checkPinID     int
	checkCode      string
	checkClientID  string
	checkProduct   string
}

func (s *stubPlexPINService) CreateAuthPIN(ctx context.Context, clientIdentifier, product string) (*types.PlexPIN, error) {
	s.createClientID = clientIdentifier
	s.createProduct = product
	return s.createPIN, s.createErr
}

func (s *stubPlexPINService) CheckAuthPIN(ctx context.Context, pinID int, code, clientIdentifier, product string) (*types.PlexPIN, error) {
	s.checkPinID = pinID
	s.checkCode = code
	s.checkClientID = clientIdentifier
	s.checkProduct = product
	return s.checkPIN, s.checkErr
}

func TestPlexAuthHandler_CreatePIN(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	stub := &stubPlexPINService{
		createPIN: &types.PlexPIN{
			ID:        42,
			Code:      "abc123",
			ExpiresIn: 900,
		},
	}

	handler := &PlexAuthHandler{plexService: stub}

	router := gin.New()
	router.POST("/api/plex/auth/pin", handler.CreatePIN)

	payload := map[string]string{
		"forwardUrl": "http://localhost:3000/settings",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/plex/auth/pin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response createPlexPINResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.PinID != 42 {
		t.Fatalf("pinId = %d, want 42", response.PinID)
	}
	if response.Code != "abc123" {
		t.Fatalf("code = %q, want %q", response.Code, "abc123")
	}
	if response.ExpiresIn != 900 {
		t.Fatalf("expiresIn = %d, want 900", response.ExpiresIn)
	}
	if !strings.HasPrefix(response.ClientIdentifier, "dashbrr-") {
		t.Fatalf("clientIdentifier = %q, want prefix %q", response.ClientIdentifier, "dashbrr-")
	}
	if stub.createProduct != defaultPlexProduct {
		t.Fatalf("create product = %q, want %q", stub.createProduct, defaultPlexProduct)
	}
	if stub.createClientID != response.ClientIdentifier {
		t.Fatalf("service clientIdentifier = %q, response clientIdentifier = %q", stub.createClientID, response.ClientIdentifier)
	}
	if !strings.Contains(response.AuthURL, "https://app.plex.tv/auth#?") {
		t.Fatalf("authUrl = %q, missing Plex auth base URL", response.AuthURL)
	}
	if !strings.Contains(response.AuthURL, "clientID=") || !strings.Contains(response.AuthURL, "code=abc123") {
		t.Fatalf("authUrl = %q, missing required query params", response.AuthURL)
	}
}

func TestPlexAuthHandler_GetPIN(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	stub := &stubPlexPINService{
		checkPIN: &types.PlexPIN{
			ID:        42,
			Code:      "abc123",
			ExpiresIn: 900,
			AuthToken: "plex-token",
		},
	}
	handler := &PlexAuthHandler{plexService: stub}

	router := gin.New()
	router.GET("/api/plex/auth/pin/:pinId", handler.GetPIN)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/plex/auth/pin/42?code=abc123&clientIdentifier=dashbrr-test-client",
		nil,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response plexPINStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !response.Authorized {
		t.Fatal("authorized = false, want true")
	}
	if response.AuthToken != "plex-token" {
		t.Fatalf("authToken = %q, want %q", response.AuthToken, "plex-token")
	}
	if stub.checkPinID != 42 {
		t.Fatalf("pinId = %d, want 42", stub.checkPinID)
	}
	if stub.checkCode != "abc123" {
		t.Fatalf("code = %q, want %q", stub.checkCode, "abc123")
	}
	if stub.checkClientID != "dashbrr-test-client" {
		t.Fatalf("clientIdentifier = %q, want %q", stub.checkClientID, "dashbrr-test-client")
	}
	if stub.checkProduct != defaultPlexProduct {
		t.Fatalf("product = %q, want %q", stub.checkProduct, defaultPlexProduct)
	}
}
