// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"context"
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
}

func (s *fakeAuthStore) Get(_ context.Context, key string, value interface{}) error {
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

func (s *fakeAuthStore) Set(context.Context, string, interface{}, time.Duration) error { return nil }
func (s *fakeAuthStore) Delete(context.Context, string) error                          { return nil }
func (s *fakeAuthStore) Increment(context.Context, string, int64) error                { return nil }
func (s *fakeAuthStore) CleanAndCount(context.Context, string, int64) error            { return nil }
func (s *fakeAuthStore) GetCount(context.Context, string) (int64, error)               { return 0, nil }
func (s *fakeAuthStore) Expire(context.Context, string, time.Duration) error           { return nil }
func (s *fakeAuthStore) Close() error                                                  { return nil }

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
