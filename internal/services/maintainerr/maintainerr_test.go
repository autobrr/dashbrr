// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package maintainerr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCollectionsServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/collections" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// Newer Maintainerr: string libraryId/type, mediaServerId instead of plexId,
// media capped to a preview plus a mediaCount total (#98).
func TestGetCollections_NewSchema(t *testing.T) {
	t.Parallel()

	server := newCollectionsServer(t, `[
		{
			"id": 1,
			"libraryId": "2",
			"type": "movie",
			"mediaServerId": "abc",
			"title": "Old Movies",
			"isActive": true,
			"deleteAfterDays": 30,
			"mediaCount": 42,
			"media": [
				{"id": 1, "collectionId": 1, "mediaServerId": "m1", "addDate": "2026-05-11"},
				{"id": 2, "collectionId": 1, "mediaServerId": "m2", "addDate": "2026-05-12"}
			]
		},
		{"id": 2, "libraryId": "3", "type": "show", "title": "Inactive", "isActive": false, "mediaCount": 5, "media": []}
	]`)

	service := NewMaintainerrService().(*MaintainerrService)
	collections, err := service.GetCollections(context.Background(), server.URL, "key")
	if err != nil {
		t.Fatalf("GetCollections() error = %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("len(collections) = %d, want 1", len(collections))
	}
	if collections[0].Title != "Old Movies" {
		t.Fatalf("Title = %q, want %q", collections[0].Title, "Old Movies")
	}
	if collections[0].MediaCount != 42 {
		t.Fatalf("MediaCount = %d, want 42", collections[0].MediaCount)
	}
	if collections[0].Media != nil {
		t.Fatalf("Media = %v, want nil (must not be re-serialized to clients)", collections[0].Media)
	}
}

// Older Maintainerr: no mediaCount, full media array, numeric libraryId/type.
func TestGetCollections_OldSchemaFallsBackToMediaLength(t *testing.T) {
	t.Parallel()

	server := newCollectionsServer(t, `[
		{
			"id": 1,
			"libraryId": 2,
			"type": 1,
			"plexId": 5,
			"title": "Old Movies",
			"isActive": true,
			"deleteAfterDays": 30,
			"media": [
				{"id": 1, "collectionId": 1, "plexId": 10},
				{"id": 2, "collectionId": 1, "plexId": 11},
				{"id": 3, "collectionId": 1, "plexId": 12}
			]
		}
	]`)

	service := NewMaintainerrService().(*MaintainerrService)
	collections, err := service.GetCollections(context.Background(), server.URL, "key")
	if err != nil {
		t.Fatalf("GetCollections() error = %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("len(collections) = %d, want 1", len(collections))
	}
	if collections[0].MediaCount != 3 {
		t.Fatalf("MediaCount = %d, want 3", collections[0].MediaCount)
	}
}

// A single collection object (not an array) is malformed and must error, not
// be accepted through a fallback that masks parse errors.
func TestGetCollections_NonArrayResponseErrors(t *testing.T) {
	t.Parallel()

	server := newCollectionsServer(t, `{"id": 1, "title": "Old Movies", "isActive": true, "mediaCount": 42}`)

	service := NewMaintainerrService().(*MaintainerrService)
	if _, err := service.GetCollections(context.Background(), server.URL, "key"); err == nil {
		t.Fatal("GetCollections() error = nil, want parse error")
	}
}
