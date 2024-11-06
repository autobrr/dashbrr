// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/database"
	"github.com/autobrr/dashbrr/backend/services/cache"
	"github.com/autobrr/dashbrr/backend/types"
	"github.com/autobrr/dashbrr/backend/utils"
)

type BuiltinAuthHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewBuiltinAuthHandler(db *database.DB, cache *cache.Cache) *BuiltinAuthHandler {
	return &BuiltinAuthHandler{
		db:    db,
		cache: cache,
	}
}

// CheckRegistrationStatus checks if registration is allowed (no users exist)
func (h *BuiltinAuthHandler) CheckRegistrationStatus(c *gin.Context) {
	hasUsers, err := h.db.HasUsers()
	if err != nil {
		log.Error().Err(err).Msg("failed to check existing users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"registrationEnabled": !hasUsers,
	})
}

// Register handles user registration
func (h *BuiltinAuthHandler) Register(c *gin.Context) {
	// Check if any users exist
	hasUsers, err := h.db.HasUsers()
	if err != nil {
		log.Error().Err(err).Msg("failed to check existing users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if hasUsers {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled. A user already exists."})
		return
	}

	var req types.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate password
	if err := utils.ValidatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username exists
	existingUser, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		log.Error().Err(err).Msg("failed to check username")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Check if email exists
	existingUser, err = h.db.GetUserByEmail(req.Email)
	if err != nil {
		log.Error().Err(err).Msg("failed to check email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Create user
	user := &types.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
	}

	if err := h.db.CreateUser(user); err != nil {
		log.Error().Err(err).Msg("failed to create user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Login handles user login
func (h *BuiltinAuthHandler) Login(c *gin.Context) {
	var req types.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Get user by username
	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		log.Error().Err(err).Msg("failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate session token
	sessionToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate session token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Create session
	expiresAt := time.Now().Add(24 * time.Hour)
	sessionData := types.SessionData{
		AccessToken: sessionToken,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		UserID:      user.ID,
		AuthType:    "builtin",
	}

	// Store session in Redis
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	if err := h.cache.Set(c, sessionKey, sessionData, time.Until(expiresAt)); err != nil {
		log.Error().Err(err).Msg("failed to store session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Set session cookie
	c.SetCookie(
		"session",
		sessionToken,
		int(time.Until(expiresAt).Seconds()),
		"/",
		"",
		true, // Secure
		true, // HttpOnly
	)

	c.JSON(http.StatusOK, gin.H{
		"access_token": sessionToken,
		"token_type":   "Bearer",
		"expires_in":   int(time.Until(expiresAt).Seconds()),
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Verify verifies the session token
func (h *BuiltinAuthHandler) Verify(c *gin.Context) {
	// Get session cookie
	sessionToken, err := c.Cookie("session")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Get session from Redis
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	var sessionData types.SessionData
	if err := h.cache.Get(c, sessionKey, &sessionData); err != nil {
		log.Error().Err(err).Msg("failed to get session")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found or expired"})
		return
	}

	// Check if session is expired
	if time.Now().After(sessionData.ExpiresAt) {
		_ = h.cache.Delete(c, sessionKey)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token is valid",
		"user_id": sessionData.UserID,
	})
}

// Logout handles user logout
func (h *BuiltinAuthHandler) Logout(c *gin.Context) {
	// Get session cookie
	sessionToken, err := c.Cookie("session")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Already logged out"})
		return
	}

	// Delete session from Redis
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
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

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetUserInfo returns the current user's information
func (h *BuiltinAuthHandler) GetUserInfo(c *gin.Context) {
	// Get session cookie
	sessionToken, err := c.Cookie("session")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Get session from Redis
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	var sessionData types.SessionData
	if err := h.cache.Get(c, sessionKey, &sessionData); err != nil {
		log.Error().Err(err).Msg("failed to get session")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		return
	}

	// Get user from database
	user, err := h.db.GetUserByID(sessionData.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}
