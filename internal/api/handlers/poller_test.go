// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
)

func TestPollerMaybeRun_CanceledContextBeforeSemaphoreAcquireClearsInFlight(t *testing.T) {
	p := NewPoller(nil, nil)

	sem := make(chan struct{}, 1)
	sem <- struct{}{} // fill; acquisition should block

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := models.ServiceConfiguration{
		InstanceID: "plex-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "plex_sessions"
	key := svc.InstanceID + ":" + job

	p.maybeRun(ctx, sem, svc, "plex", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) {})

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if !inFlight {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected inFlight %q to be cleared after ctx cancellation", key)
}

func TestPollerMaybeRun_SemaphoreFullSkipsWithoutMarkingLastRun(t *testing.T) {
	p := NewPoller(nil, nil)

	sem := make(chan struct{}, 1)
	sem <- struct{}{} // full semaphore should skip this cycle immediately

	svc := models.ServiceConfiguration{
		InstanceID: "plex-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "plex_sessions"
	key := svc.InstanceID + ":" + job

	p.maybeRun(context.Background(), sem, svc, "plex", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) {
		t.Fatalf("job should not run when semaphore is full")
	})

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		inFlight := p.inFlight[key]
		lastRun, hasLastRun := p.lastRun[key]
		p.mu.Unlock()

		if !inFlight {
			if hasLastRun && !lastRun.IsZero() {
				t.Fatalf("expected lastRun to stay unset when job is skipped, got %v", lastRun)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected inFlight %q to be cleared when semaphore is full", key)
}
