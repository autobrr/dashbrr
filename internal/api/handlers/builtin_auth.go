// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/api/session"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/internal/utils"
)

type BuiltinAuthHandler struct {
	db       *database.DB
	sessions *session.Manager
}

func NewBuiltinAuthHandler(db *database.DB, store cache.Store) *BuiltinAuthHandler {
	return &BuiltinAuthHandler{
		db:       db,
		sessions: session.New(store),
	}
}

// CheckRegistrationStatus checks if registration is allowed (no users exist)
func (h *BuiltinAuthHandler) CheckRegistrationStatus(c *gin.Context) {
	hasUsers, err := h.db.HasUsers(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to check existing users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hasUsers":            hasUsers,
		"registrationEnabled": !hasUsers,
	})
}

// Register handles user registration
func (h *BuiltinAuthHandler) Register(c *gin.Context) {
	// Check if any users exist
	hasUsers, err := h.db.HasUsers(c.Request.Context())
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
	existingUser, err := h.db.FindUser(c.Request.Context(), types.FindUserParams{Username: req.Username})
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
	existingUser, err = h.db.FindUser(c.Request.Context(), types.FindUserParams{Email: req.Email})
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

	if err := h.db.CreateUser(c.Request.Context(), user); err != nil {
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
	ctx := c.Request.Context()

	var req types.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Get user by username
	user, err := h.db.FindUser(ctx, types.FindUserParams{Username: req.Username})
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

	sessionToken, err := h.sessions.Issue(c, types.SessionData{
		UserID:   user.ID,
		AuthType: "builtin",
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": sessionToken,
		"token_type":   "Bearer",
		"expires_in":   int(session.TTL.Seconds()),
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Verify verifies the session token
func (h *BuiltinAuthHandler) Verify(c *gin.Context) {
	sessionData, ok := loadSessionOfType(c, h.sessions, "builtin")
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token is valid",
		"user_id": sessionData.UserID,
	})
}

// Logout handles user logout
func (h *BuiltinAuthHandler) Logout(c *gin.Context) {
	if err := h.sessions.Clear(c); err != nil {
		log.Error().Err(err).Msg("failed to delete session from cache")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetUserInfo returns the current user's information
func (h *BuiltinAuthHandler) GetUserInfo(c *gin.Context) {
	_, sessionData, err := h.sessions.Load(c)
	if err != nil {
		middleware.AbortWithSessionError(c, err)
		return
	}

	// Get user from database
	user, err := h.db.FindUser(c.Request.Context(), types.FindUserParams{ID: sessionData.UserID})
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
