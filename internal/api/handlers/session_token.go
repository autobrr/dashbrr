// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

var errNoSessionToken = errors.New("no session token")

// getSessionToken extracts the session token from either the "session" cookie
// or a Bearer Authorization header.
func getSessionToken(c *gin.Context) (string, error) {
	if token, err := c.Cookie("session"); err == nil && token != "" {
		return token, nil
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", errNoSessionToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" || parts[1] == "" {
		return "", errNoSessionToken
	}

	return parts[1], nil
}
