// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package autobrr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetReleases_UsesBoundedRecentQuery(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/release" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/release")
		}

		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("limit = %q, want 5", got)
		}
		if got := r.URL.Query().Get("offset"); got != "0" {
			t.Fatalf("offset = %q, want 0", got)
		}
		if got := r.Header.Get("X-Api-Token"); got != "token" {
			t.Fatalf("X-Api-Token = %q, want token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"release"}],"count":1,"next_cursor":0}`))
	}))
	defer ts.Close()

	service := &AutobrrService{}
	releases, err := service.GetReleases(context.Background(), ts.URL, "token")
	if err != nil {
		t.Fatalf("GetReleases failed: %v", err)
	}

	if len(releases.Data) != 1 {
		t.Fatalf("release data length = %d, want 1", len(releases.Data))
	}
}
