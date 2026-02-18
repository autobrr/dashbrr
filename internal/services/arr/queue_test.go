package arr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildQueueDeleteURL(t *testing.T) {
	got := BuildQueueDeleteURL("http://localhost:7878/", "123", QueueDeleteOptions{
		RemoveFromClient: true,
		Blocklist:        false,
		SkipRedownload:   true,
		ChangeCategory:   true,
	})

	want := "http://localhost:7878/api/v3/queue/123?removeFromClient=true&blocklist=false&skipRedownload=true&changeCategory=true"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeleteQueueItem_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantMsg string
	}{
		{name: "missing url", baseURL: "", apiKey: "key", wantMsg: "URL is required"},
		{name: "missing api key", baseURL: "http://localhost:7878", apiKey: "", wantMsg: "API key is required"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := DeleteQueueItem(context.Background(), "radarr", tt.baseURL, tt.apiKey, "123", QueueDeleteOptions{}, nil)
			var arrErr *ErrArr
			if !errors.As(err, &arrErr) {
				t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
			}
			if arrErr.Err == nil || arrErr.Err.Error() != tt.wantMsg {
				t.Fatalf("unexpected validation error: %v", arrErr.Err)
			}
		})
	}
}

func TestDeleteQueueItem_UpstreamMessage(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v3/queue/123" {
			t.Fatalf("path = %s, want /api/v3/queue/123", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("removeFromClient") != "true" || q.Get("blocklist") != "true" || q.Get("skipRedownload") != "false" || q.Get("changeCategory") != "true" {
			t.Fatalf("unexpected query params: %s", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"queue locked"}`))
	}))
	defer ts.Close()

	err := DeleteQueueItem(context.Background(), "radarr", ts.URL, "key", "123", QueueDeleteOptions{
		RemoveFromClient: true,
		Blocklist:        true,
		SkipRedownload:   false,
		ChangeCategory:   true,
	}, nil)

	var arrErr *ErrArr
	if !errors.As(err, &arrErr) {
		t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
	}
	if arrErr.HttpCode != http.StatusBadRequest {
		t.Fatalf("HttpCode = %d, want %d", arrErr.HttpCode, http.StatusBadRequest)
	}
	if arrErr.Err == nil || arrErr.Err.Error() != "queue locked" {
		t.Fatalf("message = %v, want %q", arrErr.Err, "queue locked")
	}
}
