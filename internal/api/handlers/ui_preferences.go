// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
)

type UIPreferencesHandler struct {
	db *database.DB
}

type collapsePreferenceRequest struct {
	Key       string `json:"key" binding:"required"`
	Collapsed bool   `json:"collapsed"`
}

func NewUIPreferencesHandler(db *database.DB) *UIPreferencesHandler {
	return &UIPreferencesHandler{db: db}
}

func (h *UIPreferencesHandler) GetCollapsePreferences(c *gin.Context) {
	userID, ok := getContextUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	preferences, err := h.db.GetUICollapsePreferences(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("Failed to load UI collapse preferences")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load UI preferences"})
		return
	}

	c.JSON(http.StatusOK, preferences)
}

func (h *UIPreferencesHandler) UpsertCollapsePreference(c *gin.Context) {
	userID, ok := getContextUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	var req collapsePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Preference key is required"})
		return
	}
	if len(key) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Preference key too long"})
		return
	}

	if err := h.db.UpsertUICollapsePreference(c.Request.Context(), userID, key, req.Collapsed); err != nil {
		log.Error().
			Err(err).
			Int64("user_id", userID).
			Str("key", key).
			Bool("collapsed", req.Collapsed).
			Msg("Failed to persist UI collapse preference")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save UI preference"})
		return
	}

	c.Status(http.StatusNoContent)
}

func getContextUserID(c *gin.Context) (int64, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		// OIDC sessions have no local DB user; RequireAuth has already
		// validated the session, so treat them as a shared user (0)
		// instead of failing with a 401 the frontend mistakes for an
		// expired session (login loop).
		if authType, ok := c.Get("auth_type"); ok && authType == "oidc" {
			return 0, true
		}
		return 0, false
	}

	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
