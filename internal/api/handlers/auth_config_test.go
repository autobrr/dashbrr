// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		hasOIDC     bool
		wantOIDC    bool
		wantBuiltin bool
		wantDefault string
	}{
		{"oidc configured", true, true, false, "oidc"},
		{"oidc not configured", false, false, true, "builtin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DASHBRR_AUTH_BYPASS", "false")

			r := gin.New()
			r.GET("/api/auth/config", AuthConfig(tt.hasOIDC))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/auth/config", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			var got struct {
				Methods map[string]bool `json:"methods"`
				Default string          `json:"default"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}

			if got.Methods["oidc"] != tt.wantOIDC {
				t.Errorf("methods.oidc = %v, want %v", got.Methods["oidc"], tt.wantOIDC)
			}
			if got.Methods["builtin"] != tt.wantBuiltin {
				t.Errorf("methods.builtin = %v, want %v", got.Methods["builtin"], tt.wantBuiltin)
			}
			if got.Default != tt.wantDefault {
				t.Errorf("default = %q, want %q", got.Default, tt.wantDefault)
			}
		})
	}
}
