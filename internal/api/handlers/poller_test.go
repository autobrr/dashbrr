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

func TestPollerMaybeRun_SemaphoreFullWaitsThenRuns(t *testing.T) {
	p := NewPoller(nil, nil)

	sem := make(chan struct{}, 1)
	sem <- struct{}{} // fill; acquisition should block until released

	svc := models.ServiceConfiguration{
		InstanceID: "plex-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "plex_sessions"
	key := svc.InstanceID + ":" + job
	ran := make(chan struct{}, 1)

	p.maybeRun(context.Background(), sem, svc, "plex", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) {
		ran <- struct{}{}
	})

	// Confirm job remains in-flight while waiting for worker slot.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if inFlight {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Release one slot; blocked job should run.
	<-sem

	select {
	case <-ran:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected blocked job to run after semaphore slot release")
	}

	deadline = time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		_, inFlight := p.inFlight[key]
		lastRun := p.lastRun[key]
		p.mu.Unlock()

		if !inFlight {
			if lastRun.IsZero() {
				t.Fatalf("expected lastRun to be set after delayed run")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected inFlight %q to be cleared after run", key)
}
