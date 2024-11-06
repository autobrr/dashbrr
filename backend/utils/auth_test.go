// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "TestPass123!",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt allows empty passwords
		},
		{
			name:     "long password",
			password: strings.Repeat("a", 72), // bcrypt's max length
			wantErr:  false,
		},
		{
			name:     "too long password",
			password: strings.Repeat("a", 73), // exceeds bcrypt's max length
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify the hash works with CheckPassword
				if !CheckPassword(tt.password, hash) {
					t.Errorf("CheckPassword() failed to verify hashed password")
				}
				// Verify wrong password fails
				if CheckPassword("wrongpassword", hash) {
					t.Errorf("CheckPassword() incorrectly verified wrong password")
				}
			}
		})
	}
}

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "valid length",
			length:  32,
			wantErr: false,
		},
		{
			name:    "minimum length",
			length:  1,
			wantErr: false,
		},
		{
			name:    "zero length",
			length:  0,
			wantErr: true,
		},
		{
			name:    "negative length",
			length:  -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token1, err1 := GenerateSecureToken(tt.length)
			if tt.wantErr {
				assert.Error(t, err1)
				return
			}
			assert.NoError(t, err1)
			assert.Equal(t, tt.length, len(token1))

			// Test uniqueness by generating another token
			token2, err2 := GenerateSecureToken(tt.length)
			assert.NoError(t, err2)
			assert.Equal(t, tt.length, len(token2))
			assert.NotEqual(t, token1, token2, "Generated tokens should be unique")

			// Verify tokens are URL-safe base64 encoded
			assert.NotContains(t, token1, "+", "Token should not contain '+'")
			assert.NotContains(t, token1, "/", "Token should not contain '/'")
			assert.NotContains(t, token2, "+", "Token should not contain '+'")
			assert.NotContains(t, token2, "/", "Token should not contain '/'")
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "TestPass123!",
			wantErr:  false,
		},
		{
			name:     "too short",
			password: "Test1!",
			wantErr:  true,
		},
		{
			name:     "no uppercase",
			password: "testpass123!",
			wantErr:  true,
		},
		{
			name:     "no lowercase",
			password: "TESTPASS123!",
			wantErr:  true,
		},
		{
			name:     "no numbers",
			password: "TestPass!!",
			wantErr:  true,
		},
		{
			name:     "no special chars",
			password: "TestPass123",
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePassword() expected error for password: %v", tt.password)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePassword() unexpected error: %v", err)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "TestPass123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name        string
		password    string
		hash        string
		wantMatches bool
	}{
		{
			name:        "correct password",
			password:    password,
			hash:        hash,
			wantMatches: true,
		},
		{
			name:        "incorrect password",
			password:    "WrongPass123!",
			hash:        hash,
			wantMatches: false,
		},
		{
			name:        "empty password",
			password:    "",
			hash:        hash,
			wantMatches: false,
		},
		{
			name:        "invalid hash",
			password:    password,
			hash:        "invalid_hash",
			wantMatches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPassword(tt.password, tt.hash); got != tt.wantMatches {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.wantMatches)
			}
		})
	}
}
