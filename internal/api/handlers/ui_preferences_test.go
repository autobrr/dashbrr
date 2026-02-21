// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/database"
)

func setupUIPreferencesTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := tempDir + "/ui_preferences_test.db"

	_ = os.Setenv("DASHBRR__DB_TYPE", "sqlite")
	_ = os.Setenv("DASHBRR__DB_PATH", dbPath)

	db, err := database.InitDBWithConfig(database.NewConfig())
	if err != nil {
		t.Fatalf("failed to initialize test DB: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.Unsetenv("DASHBRR__DB_TYPE")
		_ = os.Unsetenv("DASHBRR__DB_PATH")
	}

	return db, cleanup
}

func TestUIPreferencesHandler_CollapseRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	handler := NewUIPreferencesHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(42))
		c.Next()
	})
	router.GET("/api/ui/preferences/collapse", handler.GetCollapsePreferences)
	router.PUT("/api/ui/preferences/collapse", handler.UpsertCollapsePreference)

	getEmpty := httptest.NewRecorder()
	getEmptyReq, _ := http.NewRequest(http.MethodGet, "/api/ui/preferences/collapse", nil)
	router.ServeHTTP(getEmpty, getEmptyReq)

	if getEmpty.Code != http.StatusOK {
		t.Fatalf("expected empty GET status 200, got %d", getEmpty.Code)
	}

	var empty map[string]bool
	if err := json.Unmarshal(getEmpty.Body.Bytes(), &empty); err != nil {
		t.Fatalf("failed to decode empty response: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty preference map, got %d entries", len(empty))
	}

	putBody := `{"key":"service:autobrr-1:section:recent_releases","collapsed":true}`
	put := httptest.NewRecorder()
	putReq, _ := http.NewRequest(http.MethodPut, "/api/ui/preferences/collapse", strings.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(put, putReq)

	if put.Code != http.StatusNoContent {
		t.Fatalf("expected PUT status 204, got %d body=%s", put.Code, put.Body.String())
	}

	get := httptest.NewRecorder()
	getReq, _ := http.NewRequest(http.MethodGet, "/api/ui/preferences/collapse", nil)
	router.ServeHTTP(get, getReq)

	if get.Code != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d", get.Code)
	}

	var prefs map[string]bool
	if err := json.Unmarshal(get.Body.Bytes(), &prefs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !prefs["service:autobrr-1:section:recent_releases"] {
		t.Fatalf("expected persisted collapse preference to be true")
	}
}

func TestUIPreferencesHandler_RejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	handler := NewUIPreferencesHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(7))
		c.Next()
	})
	router.PUT("/api/ui/preferences/collapse", handler.UpsertCollapsePreference)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/ui/preferences/collapse", strings.NewReader(`{"collapsed":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}
