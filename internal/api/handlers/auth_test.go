// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/autobrr/dashbrr/internal/types"
)

// MockCache is a mock implementation of CacheInterface
type MockCache struct {
	mock.Mock
}

// safeArgs ensures we always return a valid mock.Arguments
func (m *MockCache) safeArgs(args mock.Arguments) mock.Arguments {
	if args == nil {
		return mock.Arguments{errors.New("mock not configured")}
	}
	return args
}

func (m *MockCache) Get(ctx context.Context, key string, value interface{}) error {
	args := m.safeArgs(m.Called(ctx, key, value))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.safeArgs(m.Called(ctx, key, value, expiration))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.safeArgs(m.Called(ctx, key))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) Increment(ctx context.Context, key string, timestamp int64) error {
	args := m.safeArgs(m.Called(ctx, key, timestamp))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	args := m.safeArgs(m.Called(ctx, key, windowStart))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) GetCount(ctx context.Context, key string) (int64, error) {
	args := m.safeArgs(m.Called(ctx, key))
	var count int64
	if args.Get(0) != nil {
		if c, ok := args.Get(0).(int64); ok {
			count = c
		}
	}
	var err error
	if args.Get(1) != nil {
		if e, ok := args.Get(1).(error); ok {
			err = e
		}
	}
	return count, err
}

func (m *MockCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.safeArgs(m.Called(ctx, key, expiration))
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func (m *MockCache) Close() error {
	args := m.safeArgs(m.Called())
	if args.Get(0) == nil {
		return nil
	}
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return errors.New("unknown error")
}

func TestNewAuthHandler(t *testing.T) {
	config := &types.AuthConfig{
		Issuer:       "https://test.auth0.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
	}
	mockCache := new(MockCache)

	handler := NewAuthHandler(config, mockCache)

	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.NotNil(t, handler.oauth2Config)
	assert.Equal(t, "test-client-id", handler.oauth2Config.ClientID)
	assert.Equal(t, "test-client-secret", handler.oauth2Config.ClientSecret)
	assert.Equal(t, "http://localhost:3000/callback", handler.oauth2Config.RedirectURL)
}

func TestLogin_NoFrontendURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/login", nil)
	c.Request = req

	mockCache := new(MockCache)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		cache: mockCache,
	}

	handler.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockCache.AssertExpectations(t)
}

func TestCallback_NoCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/callback", nil)
	c.Request = req

	mockCache := new(MockCache)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		config: &types.AuthConfig{
			Issuer:       "https://test.auth0.com",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		cache: mockCache,
	}

	handler.Callback(c)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=no_code")
	mockCache.AssertExpectations(t)
}

func TestLogout_NoFrontendURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/logout", nil)
	c.Request = req

	mockCache := new(MockCache)
	// No mock expectations needed for this test as no cache methods are called

	handler := &AuthHandler{
		config: &types.AuthConfig{
			Issuer:       "https://test.auth0.com",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		cache: mockCache,
	}

	handler.Logout(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockCache.AssertExpectations(t)
}
