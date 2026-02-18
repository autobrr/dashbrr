package arr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetArrSystemStatusWithVersion_UsesRequestedAPIVersion(t *testing.T) {
	t.Parallel()

	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
	}))
	defer ts.Close()

	cacheVersion := func(context.Context, string, string, time.Duration) error { return nil }
	getVersionFromCache := func(context.Context, string) string { return "" }

	version, err := GetArrSystemStatusWithVersion(
		context.Background(),
		"prowlarr",
		"v1",
		ts.URL,
		"key",
		getVersionFromCache,
		cacheVersion,
	)
	if err != nil {
		t.Fatalf("GetArrSystemStatusWithVersion failed: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want %q", version, "1.2.3")
	}
	if requestedPath != "/api/v1/system/status" {
		t.Fatalf("path = %q, want %q", requestedPath, "/api/v1/system/status")
	}
}

func TestCheckArrForUpdatesWithVersion_UsesRequestedAPIVersion(t *testing.T) {
	t.Parallel()

	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"installed":false,"installable":true}]`))
	}))
	defer ts.Close()

	hasUpdate, err := CheckArrForUpdatesWithVersion(context.Background(), "prowlarr", "v1", ts.URL, "key")
	if err != nil {
		t.Fatalf("CheckArrForUpdatesWithVersion failed: %v", err)
	}
	if !hasUpdate {
		t.Fatalf("hasUpdate = false, want true")
	}
	if requestedPath != "/api/v1/update" {
		t.Fatalf("path = %q, want %q", requestedPath, "/api/v1/update")
	}
}

func TestCheckArrForUpdates_DefaultsToV3(t *testing.T) {
	t.Parallel()

	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	_, err := CheckArrForUpdatesWithVersion(context.Background(), "radarr", "", ts.URL, "key")
	if err != nil {
		t.Fatalf("CheckArrForUpdatesWithVersion failed: %v", err)
	}
	if requestedPath != "/api/v3/update" {
		t.Fatalf("path = %q, want %q", requestedPath, "/api/v3/update")
	}
}
