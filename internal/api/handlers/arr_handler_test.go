package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/services/arr"
)

func newJSONContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = req
	return c, w
}

func readErrorBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	return payload["error"]
}

func TestHandleArrFetchError_ServiceNotConfigured(t *testing.T) {
	t.Parallel()

	c, w := newJSONContext()
	handled := handleArrFetchError(c, NewServiceNotConfigured("sonarr"), "Sonarr", "sonarr-1", "queue")
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if got := readErrorBody(t, w); got == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestHandleArrFetchError_ArrHTTPCodeMapsToUpstreamStatus(t *testing.T) {
	t.Parallel()

	c, w := newJSONContext()
	err := &arr.ErrArr{Service: "radarr", Op: "get_queue", HttpCode: http.StatusUnauthorized}
	handled := handleArrFetchError(c, err, "Radarr", "radarr-1", "queue")
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestHandleArrFetchError_FallsBackToInternalServerError(t *testing.T) {
	t.Parallel()

	c, w := newJSONContext()
	handled := handleArrFetchError(c, fmt.Errorf("boom"), "Sonarr", "sonarr-1", "stats")
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if got := readErrorBody(t, w); got == "" {
		t.Fatalf("expected non-empty error message")
	}
}
