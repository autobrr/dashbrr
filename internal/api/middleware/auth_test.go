// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

type fakeAuthStore struct {
	sessions map[string]types.SessionData
	getFn    func(ctx context.Context, key string, value any) error
}

func (s *fakeAuthStore) Get(ctx context.Context, key string, value any) error {
	if s.getFn != nil {
		return s.getFn(ctx, key, value)
	}

	session, ok := s.sessions[key]
	if !ok {
		return cache.ErrKeyNotFound
	}

	out, ok := value.(*types.SessionData)
	if !ok {
		return nil
	}
	*out = session
	return nil
}

func (s *fakeAuthStore) Set(context.Context, string, any, time.Duration) error { return nil }
func (s *fakeAuthStore) Delete(context.Context, string) error                  { return nil }
func (s *fakeAuthStore) Increment(context.Context, string, int64) error        { return nil }
func (s *fakeAuthStore) CleanAndCount(context.Context, string, int64) error    { return nil }
func (s *fakeAuthStore) GetCount(context.Context, string) (int64, error)       { return 0, nil }
func (s *fakeAuthStore) Expire(context.Context, string, time.Duration) error   { return nil }
func (s *fakeAuthStore) Close() error                                          { return nil }

func TestRequireAuth_DoesNotInjectLookupTimeoutIntoRequestContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := &fakeAuthStore{
		sessions: map[string]types.SessionData{
			"session:test-token": {UserID: 42, AuthType: "builtin"},
		},
	}

	auth := NewAuthMiddleware(store)
	router := gin.New()
	router.Use(auth.RequireAuth())

	router.GET("/api/events", func(c *gin.Context) {
		if _, hasDeadline := c.Request.Context().Deadline(); hasDeadline {
			t.Fatalf("expected no deadline on downstream request context")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-token"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOptionalAuth_DoesNotInjectLookupTimeoutIntoRequestContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := &fakeAuthStore{
		sessions: map[string]types.SessionData{
			"session:test-token": {UserID: 7, AuthType: "builtin"},
		},
	}

	auth := NewAuthMiddleware(store)
	router := gin.New()
	router.Use(auth.OptionalAuth())

	router.GET("/api/events", func(c *gin.Context) {
		if _, hasDeadline := c.Request.Context().Deadline(); hasDeadline {
			t.Fatalf("expected no deadline on downstream request context")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-token"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOptionalAuth_AcceptsBearerAuthorization(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := &fakeAuthStore{
		sessions: map[string]types.SessionData{
			"session:test-token": {UserID: 7, AuthType: "builtin"},
		},
	}

	auth := NewAuthMiddleware(store)
	router := gin.New()
	router.Use(auth.OptionalAuth())

	router.GET("/api/events", func(c *gin.Context) {
		userID, ok := c.Get("user_id")
		if !ok || userID.(int64) != 7 {
			t.Fatalf("expected bearer-authenticated user context")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Authorization", "Bearer   test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAuth_DoesNotMaskSessionLookupErrorsAsUnauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := &fakeAuthStore{
		getFn: func(_ context.Context, key string, _ any) error {
			if key == "oidc:session:test-token" {
				return errors.New("cache unavailable")
			}
			return cache.ErrKeyNotFound
		},
	}

	auth := NewAuthMiddleware(store)
	router := gin.New()
	router.Use(auth.RequireAuth())
	router.GET("/api/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-token"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}
