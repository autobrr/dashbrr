// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerBasicAuth(t *testing.T) {
	users := map[string]string{"prometheus": "se:cret"}

	tests := []struct {
		name     string
		users    map[string]string
		user     string
		password string
		noAuth   bool
		want     int
	}{
		{name: "no credentials", users: users, noAuth: true, want: http.StatusUnauthorized},
		{name: "wrong password", users: users, user: "prometheus", password: "wrong", want: http.StatusUnauthorized},
		{name: "wrong user", users: users, user: "grafana", password: "se:cret", want: http.StatusUnauthorized},
		{name: "correct credentials", users: users, user: "prometheus", password: "se:cret", want: http.StatusOK},
		{name: "auth off", users: nil, noAuth: true, want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler(NewCollector(fakeServices(), fakeLatest()), tt.users)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
			if !tt.noAuth {
				req.SetBasicAuth(tt.user, tt.password)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got != `Basic realm="metrics"` {
					t.Errorf("WWW-Authenticate = %q, want Basic realm=\"metrics\"", got)
				}
				return
			}
			if !strings.Contains(rec.Body.String(), "dashbrr_info") {
				t.Error("body has no dashbrr_info series")
			}
		})
	}
}
