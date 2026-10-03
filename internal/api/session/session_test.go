// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

func newTestContext(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func newMemoryStore(t *testing.T, dir string) cache.Store {
	t.Helper()
	store := cache.NewMemoryStore(t.Context(), dir)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func responseCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == CookieName {
			return cookie
		}
	}
	t.Fatalf("response has no %s cookie", CookieName)
	return nil
}

func TestIssue(t *testing.T) {
	for _, tc := range []struct {
		proto      string
		wantSecure bool
	}{
		{proto: "https", wantSecure: true},
		{proto: "http", wantSecure: false},
	} {
		t.Run(tc.proto, func(t *testing.T) {
			store := newMemoryStore(t, t.TempDir())
			m := New(store)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/login", nil)
			req.Header.Set("X-Forwarded-Proto", tc.proto)
			c, w := newTestContext(t, req)

			token, err := m.Issue(c, types.SessionData{UserID: 5, AuthType: "builtin"})
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}

			var stored types.SessionData
			if err := store.Get(t.Context(), "session:"+token, &stored); err != nil {
				t.Fatalf("session not stored: %v", err)
			}
			if stored.UserID != 5 || stored.ExpiresAt.IsZero() {
				t.Fatalf("stored session = %+v", stored)
			}

			cookie := responseCookie(t, w)
			if cookie.Value != token || !cookie.HttpOnly || cookie.Secure != tc.wantSecure {
				t.Fatalf("cookie = %+v, want value %q secure %v", cookie, token, tc.wantSecure)
			}
		})
	}
}

func TestClear(t *testing.T) {
	store := newMemoryStore(t, t.TempDir())
	m := New(store)

	c, _ := newTestContext(t, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	token, err := m.Issue(c, types.SessionData{UserID: 1, AuthType: "builtin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: token}) //nolint:gosec // request cookie; attributes don't apply
	c, w := newTestContext(t, req)
	if err := m.Clear(c); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	var stored types.SessionData
	if err := store.Get(t.Context(), "session:"+token, &stored); !errors.Is(err, cache.ErrKeyNotFound) {
		t.Fatalf("session still stored: err = %v", err)
	}
	if cookie := responseCookie(t, w); cookie.MaxAge >= 0 {
		t.Fatalf("cookie not expired: %+v", cookie)
	}
}

func TestClear_NoSession(t *testing.T) {
	m := New(newMemoryStore(t, t.TempDir()))
	c, w := newTestContext(t, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/logout", nil))

	if err := m.Clear(c); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if cookie := responseCookie(t, w); cookie.MaxAge >= 0 {
		t.Fatalf("cookie not expired: %+v", cookie)
	}
}

func TestLoad(t *testing.T) {
	store := newMemoryStore(t, t.TempDir())
	m := New(store)
	if err := store.Set(t.Context(), "session:good", types.SessionData{UserID: 9, AuthType: "oidc"}, TTL); err != nil {
		t.Fatalf("Set: %v", err)
	}

	for _, tc := range []struct {
		name    string
		cookie  *string
		header  string
		wantErr error
	}{
		{name: "cookie", cookie: new("good")},
		{name: "bearer", header: "Bearer good"},
		{name: "bearer case and spaces", header: "bEaReR   good"},
		{name: "empty cookie falls through to bearer", cookie: new(""), header: "Bearer good"},
		{name: "no token", wantErr: ErrNoToken},
		{name: "empty cookie only", cookie: new(""), wantErr: ErrNoToken},
		{name: "malformed header", header: "Bearer", wantErr: ErrNoToken},
		{name: "wrong scheme", header: "Basic good", wantErr: ErrNoToken},
		{name: "extra fields", header: "Bearer good extra", wantErr: ErrNoToken},
		{name: "unknown token", cookie: new("missing"), wantErr: cache.ErrKeyNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tc.cookie != nil {
				req.AddCookie(&http.Cookie{Name: CookieName, Value: *tc.cookie}) //nolint:gosec // request cookie; attributes don't apply
			}
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			c, _ := newTestContext(t, req)

			token, data, err := m.Load(c)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if token != "good" || data.UserID != 9 || data.AuthType != "oidc" {
				t.Fatalf("Load = %q, %+v", token, data)
			}
		})
	}
}

func TestLoad_ReturnsContextErrorOnTimeout(t *testing.T) {
	m := New(blockingStore{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "token"}) //nolint:gosec // request cookie; attributes don't apply
	c, _ := newTestContext(t, req)

	if _, _, err := m.Load(c); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// A session must survive a restart. This fails when the session key prefix
// and the MemoryStore persistence filter drift apart.
func TestIssuedSessionSurvivesStoreRestart(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewMemoryStore(t.Context(), dir)

	c, _ := newTestContext(t, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	token, err := New(store).Issue(c, types.SessionData{UserID: 3, AuthType: "builtin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: token}) //nolint:gosec // request cookie; attributes don't apply
	c, _ = newTestContext(t, req)

	_, data, err := New(newMemoryStore(t, dir)).Load(c)
	if err != nil {
		t.Fatalf("Load after restart: %v", err)
	}
	if data.UserID != 3 {
		t.Fatalf("UserID = %d, want 3", data.UserID)
	}
}

type blockingStore struct{ cache.Store }

// Get returns its own error when the context ends, so the test proves that
// Load reports the context error and not the store error.
func (blockingStore) Get(ctx context.Context, _ string, _ any) error {
	<-ctx.Done()
	return errors.New("store gave up")
}
