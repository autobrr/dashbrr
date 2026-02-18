package sse

import (
	"context"
	"testing"
	"time"
)

func waitForCount(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.SubscriberCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := hub.SubscriberCount(); got != want {
		t.Fatalf("SubscriberCount = %d, want %d", got, want)
	}
}

func TestHubSubscriberCountLifecycle(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = hub.Subscribe(ctx, 1)
	waitForCount(t, hub, 1)

	cancel()
	waitForCount(t, hub, 0)
}

func TestHubCloseClearsSubscribers(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	_, _ = hub.Subscribe(ctx1, 1)
	_, _ = hub.Subscribe(ctx2, 1)
	waitForCount(t, hub, 2)

	hub.Close()
	waitForCount(t, hub, 0)
}
