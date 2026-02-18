package arr

import (
	"context"
	"errors"
	"io"
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

func TestBuildQueueURL(t *testing.T) {
	got := BuildQueueURL("http://localhost:7878/", "page=1&pageSize=10")
	want := "http://localhost:7878/api/v3/queue?page=1&pageSize=10"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = BuildQueueURL("http://localhost:7878/", "?page=2")
	want = "http://localhost:7878/api/v3/queue?page=2"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFetchQueueBody_Validation(t *testing.T) {
	t.Parallel()

	readBody := func(_ *http.Response) ([]byte, error) { return []byte("[]"), nil }

	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		reader  func(*http.Response) ([]byte, error)
		wantMsg string
	}{
		{name: "missing url", baseURL: "", apiKey: "key", reader: readBody, wantMsg: "URL is required"},
		{name: "missing api key", baseURL: "http://localhost:7878", apiKey: "", reader: readBody, wantMsg: "API key is required"},
		{name: "missing readBody", baseURL: "http://localhost:7878", apiKey: "key", reader: nil, wantMsg: "readBody function is required"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := FetchQueueBody(context.Background(), "radarr", tt.baseURL, tt.apiKey, "page=1", tt.reader)
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

func TestFetchQueueBody_UpstreamStatus(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	readBody := func(_ *http.Response) ([]byte, error) { return []byte("[]"), nil }
	_, err := FetchQueueBody(context.Background(), "radarr", ts.URL, "key", "page=1", readBody)

	var arrErr *ErrArr
	if !errors.As(err, &arrErr) {
		t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
	}
	if arrErr.HttpCode != http.StatusUnauthorized {
		t.Fatalf("HttpCode = %d, want %d", arrErr.HttpCode, http.StatusUnauthorized)
	}
}

func TestFetchQueueBody_Success(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v3/queue" {
			t.Fatalf("path = %s, want /api/v3/queue", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Fatalf("query page = %q, want %q", r.URL.Query().Get("page"), "1")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	readBody := func(resp *http.Response) ([]byte, error) {
		return []byte(`{"ok":true}`), nil
	}
	body, err := FetchQueueBody(context.Background(), "radarr", ts.URL, "key", "page=1", readBody)
	if err != nil {
		t.Fatalf("FetchQueueBody failed: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

type testQueueRecord struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func TestFetchQueueRecords_ParseError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"records":[`))
	}))
	defer ts.Close()

	readBody := func(resp *http.Response) ([]byte, error) {
		return io.ReadAll(resp.Body)
	}

	_, err := FetchQueueRecords[testQueueRecord](context.Background(), "radarr", ts.URL, "key", "page=1", readBody)
	var arrErr *ErrArr
	if !errors.As(err, &arrErr) {
		t.Fatalf("expected *ErrArr, got %T (%v)", err, err)
	}
	if arrErr.Op != "get_queue" {
		t.Fatalf("op = %q, want %q", arrErr.Op, "get_queue")
	}
	if arrErr.Err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestFetchQueueRecords_Success(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"records":[{"id":1,"title":"one"},{"id":2,"title":"two"}]}`))
	}))
	defer ts.Close()

	readBody := func(resp *http.Response) ([]byte, error) {
		return io.ReadAll(resp.Body)
	}

	records, err := FetchQueueRecords[testQueueRecord](context.Background(), "radarr", ts.URL, "key", "page=1", readBody)
	if err != nil {
		t.Fatalf("FetchQueueRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Title != "one" || records[1].Title != "two" {
		t.Fatalf("unexpected records: %+v", records)
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
