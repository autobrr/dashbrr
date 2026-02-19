// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

func waitForSubscriberCount(t *testing.T, hub *sse.Hub, expected int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.SubscriberCount() == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected subscriber count %d within %v", expected, timeout)
}

func TestEventsHandlerStream_ReplaysSnapshotThenStreamsLiveEvents(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hub := sse.NewHub()
	bc := NewBroadcaster(hub)
	handler := NewEventsHandler(hub, bc)

	now := time.Unix(1700000000, 0)
	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "Healthy",
		LastChecked: now,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	done := make(chan struct{})
	go func() {
		handler.Stream(c)
		close(done)
	}()

	waitForSubscriberCount(t, hub, 1, 500*time.Millisecond)

	bc.Publish(models.ServiceHealth{
		ServiceID:   "radarr-1",
		Status:      "online",
		Message:     "radarr_queue",
		LastChecked: now.Add(time.Second),
		EventType:   models.ServiceEventInternal,
	})

	time.Sleep(60 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("expected stream handler to exit after context cancellation")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "retry: 5000\n\n") {
		t.Fatalf("expected retry directive in SSE stream")
	}
	if !strings.Contains(body, `"message":"Healthy"`) {
		t.Fatalf("expected snapshot health payload in SSE stream")
	}
	if !strings.Contains(body, `"message":"radarr_queue"`) {
		t.Fatalf("expected live internal payload in SSE stream")
	}

	snapshotIdx := strings.Index(body, `"message":"Healthy"`)
	liveIdx := strings.Index(body, `"message":"radarr_queue"`)
	if snapshotIdx == -1 || liveIdx == -1 {
		t.Fatalf("expected snapshot and live payload indexes")
	}
	if snapshotIdx > liveIdx {
		t.Fatalf("expected snapshot payload before live payload")
	}
}

func TestEncodeHealthAsSSE(t *testing.T) {
	health := models.ServiceHealth{
		ServiceID:   "plex-1",
		Status:      "online",
		Message:     "Healthy",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"plex": map[string]interface{}{"sessions": []interface{}{}},
		},
	}

	payload := EncodeHealthAsSSE(health)
	encoded := string(payload)
	if !strings.HasPrefix(encoded, "data: ") {
		t.Fatalf("expected SSE payload to start with data prefix")
	}
	if !strings.HasSuffix(encoded, "\n\n") {
		t.Fatalf("expected SSE payload to end with double newline")
	}

	raw := strings.TrimSuffix(strings.TrimPrefix(encoded, "data: "), "\n\n")
	var decoded models.ServiceHealth
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode SSE payload: %v", err)
	}

	if decoded.ServiceID != health.ServiceID {
		t.Fatalf("service id = %q, want %q", decoded.ServiceID, health.ServiceID)
	}
	if decoded.Status != health.Status {
		t.Fatalf("status = %q, want %q", decoded.Status, health.Status)
	}
	if decoded.Message != health.Message {
		t.Fatalf("message = %q, want %q", decoded.Message, health.Message)
	}
}

func TestIsExpectedSSEWriteError_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if !isExpectedSSEWriteError(ctx, fmt.Errorf("write failed")) {
		t.Fatalf("expected canceled context to be treated as expected SSE write error")
	}
}

func TestIsExpectedSSEWriteError_ConnectionClosed(t *testing.T) {
	ctx := context.Background()

	if !isExpectedSSEWriteError(ctx, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed to be treated as expected SSE write error")
	}
	if !isExpectedSSEWriteError(ctx, syscall.EPIPE) {
		t.Fatalf("expected EPIPE to be treated as expected SSE write error")
	}
	if !isExpectedSSEWriteError(ctx, syscall.ECONNRESET) {
		t.Fatalf("expected ECONNRESET to be treated as expected SSE write error")
	}
}

func TestIsExpectedSSEWriteError_Unexpected(t *testing.T) {
	ctx := context.Background()

	if isExpectedSSEWriteError(ctx, fmt.Errorf("unexpected")) {
		t.Fatalf("expected generic error to be treated as unexpected")
	}
}
