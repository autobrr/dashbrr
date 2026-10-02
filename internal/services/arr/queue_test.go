// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	wantVersion := map[string]string{
		"sonarr":   "v3",
		"radarr":   "v3",
		"whisparr": "v3",
		"lidarr":   "v1",
		"readarr":  "v1",
	}
	if len(Apps) != len(wantVersion) {
		t.Fatalf("len(Apps) = %d, want %d", len(Apps), len(wantVersion))
	}

	for _, app := range Apps {
		t.Run(app.Name, func(t *testing.T) {
			t.Parallel()

			if app.APIVersion != wantVersion[app.Name] {
				t.Fatalf("APIVersion = %q, want %q", app.APIVersion, wantVersion[app.Name])
			}
			base := "/api/" + app.APIVersion + "/queue"

			var deleteQuery url.Values
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("X-Api-Key"); got != "secret" {
					t.Errorf("X-Api-Key = %q, want secret", got)
				}
				switch {
				case r.Method == http.MethodGet && r.URL.Path == base:
					if r.URL.RawQuery != "page=1&pageSize=10" {
						t.Errorf("query = %q, want page=1&pageSize=10", r.URL.RawQuery)
					}
					_, _ = w.Write([]byte(`{"page":1,"pageSize":10,"totalRecords":25,"records":[
						{"id":11,"title":"one","status":"downloading","size":1024,"sizeleft":512,
						 "timeleft":"00:10:00","protocol":"torrent","indexer":"idx","downloadClient":"qbit",
						 "customFormatScore":7,"trackedDownloadStatus":"warning","trackedDownloadState":"importPending",
						 "statusMessages":[{"title":"t","messages":["m"]}],"downloadId":"ABC"},
						{"id":12,"title":"two","status":"queued"}]}`))
				case r.Method == http.MethodDelete && r.URL.Path == base+"/42":
					deleteQuery = r.URL.Query()
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			page, err := app.FetchQueue(t.Context(), srv.URL, "secret")
			if err != nil {
				t.Fatalf("FetchQueue: %v", err)
			}
			if page.TotalRecords != 25 {
				t.Errorf("TotalRecords = %d, want 25", page.TotalRecords)
			}
			if len(page.Records) != 2 {
				t.Fatalf("len(Records) = %d, want 2", len(page.Records))
			}
			first := page.Records[0]
			if first.ID != 11 || first.Size != 1024 || first.SizeLeft != 512 ||
				first.TrackedDownloadState != "importPending" || first.CustomFormatScore != 7 ||
				first.DownloadID != "ABC" || len(first.StatusMessages) != 1 {
				t.Errorf("unexpected first record: %+v", first)
			}

			err = app.DeleteQueueItem(t.Context(), srv.URL, "secret", "42", QueueDeleteOptions{
				RemoveFromClient: true,
				Blocklist:        true,
				ChangeCategory:   true,
			})
			if err != nil {
				t.Fatalf("DeleteQueueItem: %v", err)
			}
			for key, want := range map[string]string{
				"removeFromClient": "true",
				"blocklist":        "true",
				"skipRedownload":   "false",
				"changeCategory":   "true",
			} {
				if got := deleteQuery.Get(key); got != want {
					t.Errorf("delete query %s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestAppFor(t *testing.T) {
	t.Parallel()

	if app, ok := AppFor("lidarr"); !ok || app != Lidarr {
		t.Fatalf("AppFor(lidarr) = %+v, %v", app, ok)
	}
	if _, ok := AppFor("prowlarr"); ok {
		t.Fatal("AppFor(prowlarr) = ok, want not an *arr app")
	}
}

func TestFetchQueue_EmptyRecordsIsNotNil(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"totalRecords":0,"records":null}`))
	}))
	defer srv.Close()

	page, err := Sonarr.FetchQueue(t.Context(), srv.URL, "key")
	if err != nil {
		t.Fatalf("FetchQueue: %v", err)
	}
	if page.Records == nil {
		t.Fatal("Records is nil; the web reads it as a list")
	}
}

func TestFetchQueue_Errors(t *testing.T) {
	t.Parallel()

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(unauthorized.Close)

	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"records":[`))
	}))
	t.Cleanup(malformed.Close)

	tests := []struct {
		name     string
		baseURL  string
		apiKey   string
		wantCode int
		wantMsg  string
	}{
		{name: "missing url", apiKey: "key", wantMsg: "URL is required"},
		{name: "missing api key", baseURL: "http://127.0.0.1:1", wantMsg: "API key is required"},
		{name: "upstream status", baseURL: unauthorized.URL, apiKey: "key", wantCode: http.StatusUnauthorized},
		{name: "parse error", baseURL: malformed.URL, apiKey: "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Radarr.FetchQueue(t.Context(), tt.baseURL, tt.apiKey)
			arrErr, ok := errors.AsType[*ErrArr](err)
			if !ok {
				t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
			}
			if arrErr.Service != "radarr" || arrErr.Op != "get_queue" {
				t.Fatalf("unexpected error fields: %+v", arrErr)
			}
			if arrErr.HttpCode != tt.wantCode {
				t.Fatalf("HttpCode = %d, want %d", arrErr.HttpCode, tt.wantCode)
			}
			if tt.wantMsg != "" && (arrErr.Err == nil || arrErr.Err.Error() != tt.wantMsg) {
				t.Fatalf("Err = %v, want %q", arrErr.Err, tt.wantMsg)
			}
		})
	}
}

func TestDeleteQueueItem_Errors(t *testing.T) {
	t.Parallel()

	locked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"queue locked"}`))
	}))
	t.Cleanup(locked.Close)

	tests := []struct {
		name     string
		baseURL  string
		apiKey   string
		wantCode int
		wantMsg  string
	}{
		{name: "missing url", apiKey: "key", wantMsg: "URL is required"},
		{name: "missing api key", baseURL: "http://127.0.0.1:1", wantMsg: "API key is required"},
		{name: "upstream message", baseURL: locked.URL, apiKey: "key", wantCode: http.StatusBadRequest, wantMsg: "queue locked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Whisparr.DeleteQueueItem(t.Context(), tt.baseURL, tt.apiKey, "123", QueueDeleteOptions{})
			arrErr, ok := errors.AsType[*ErrArr](err)
			if !ok {
				t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
			}
			if arrErr.Service != "whisparr" || arrErr.Op != "delete_queue" {
				t.Fatalf("unexpected error fields: %+v", arrErr)
			}
			if arrErr.HttpCode != tt.wantCode {
				t.Fatalf("HttpCode = %d, want %d", arrErr.HttpCode, tt.wantCode)
			}
			if arrErr.Err == nil || arrErr.Err.Error() != tt.wantMsg {
				t.Fatalf("Err = %v, want %q", arrErr.Err, tt.wantMsg)
			}
		})
	}
}
