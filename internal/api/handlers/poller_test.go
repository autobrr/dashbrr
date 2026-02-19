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

	p.maybeRun(ctx, sem, svc, "plex", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		return nil
	})

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

	p.maybeRun(context.Background(), sem, svc, "plex", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		ran <- struct{}{}
		return nil
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

func TestPollerMaybeRun_FailedJobsRetrySoonerThanNominalInterval(t *testing.T) {
	p := NewPoller(nil, nil)
	sem := make(chan struct{}, 1)

	svc := models.ServiceConfiguration{
		InstanceID: "sonarr-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "sonarr_queue"
	key := svc.InstanceID + ":" + job

	failed := make(chan struct{}, 1)
	p.maybeRun(context.Background(), sem, svc, "sonarr", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		failed <- struct{}{}
		return context.DeadlineExceeded
	})

	select {
	case <-failed:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected first run to fail")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		isFailed := p.failed[key]
		p.mu.Unlock()
		if isFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	p.mu.Lock()
	if !p.failed[key] {
		p.mu.Unlock()
		t.Fatalf("expected failed[%q] = true after failing run", key)
	}
	p.lastRun[key] = time.Now().Add(-pollerFailedRetryDelay - time.Millisecond)
	p.mu.Unlock()

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if !inFlight {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	retried := make(chan struct{}, 1)
	p.maybeRun(context.Background(), sem, svc, "sonarr", job, time.Hour, pollerDefaultJobTimeout, false, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		retried <- struct{}{}
		return nil
	})

	select {
	case <-retried:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected failed job retry after %v", pollerFailedRetryDelay)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failed[key] {
		t.Fatalf("expected failed[%q] = false after successful retry", key)
	}
	if p.lastOKRun[key].IsZero() {
		t.Fatalf("expected lastOKRun[%q] to be set after successful retry", key)
	}
}

func TestPollerMaybeRun_PanicMarksJobFailed(t *testing.T) {
	p := NewPoller(nil, nil)
	sem := make(chan struct{}, 1)

	svc := models.ServiceConfiguration{
		InstanceID: "radarr-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "radarr_queue"
	key := svc.InstanceID + ":" + job

	p.maybeRun(context.Background(), sem, svc, "radarr", job, time.Hour, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		panic("boom")
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		failed := p.failed[key]
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if failed && !inFlight {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected panic job to be marked failed and cleared from inFlight")
}

func TestPollerMaybeRun_FailureMarksJobStaleAfterThreshold(t *testing.T) {
	p := NewPoller(nil, nil)
	sem := make(chan struct{}, 1)

	svc := models.ServiceConfiguration{
		InstanceID: "prowlarr-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "prowlarr_stats"
	key := svc.InstanceID + ":" + job
	interval := 20 * time.Second

	p.mu.Lock()
	p.lastOKRun[key] = time.Now().Add(-pollerStaleDataThreshold(interval) - time.Second)
	p.mu.Unlock()

	p.maybeRun(context.Background(), sem, svc, "prowlarr", job, interval, pollerDefaultJobTimeout, true, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		return context.DeadlineExceeded
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		staleWarn := p.staleWarn[key]
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if staleWarn && !inFlight {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected stale warning to be set for key %q after prolonged failure", key)
}

func TestPollerMaybeRun_SuccessClearsStaleWarning(t *testing.T) {
	p := NewPoller(nil, nil)
	sem := make(chan struct{}, 1)

	svc := models.ServiceConfiguration{
		InstanceID: "prowlarr-1",
		URL:        "http://example",
		APIKey:     "key",
	}

	job := "prowlarr_stats"
	key := svc.InstanceID + ":" + job

	p.mu.Lock()
	p.staleWarn[key] = true
	p.failed[key] = true
	p.lastRun[key] = time.Now().Add(-pollerFailedRetryDelay - time.Second)
	p.mu.Unlock()

	p.maybeRun(context.Background(), sem, svc, "prowlarr", job, 20*time.Second, pollerDefaultJobTimeout, false, func(*Poller, context.Context, models.ServiceConfiguration, string) error {
		return nil
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		staleWarn := p.staleWarn[key]
		failed := p.failed[key]
		inFlight := p.inFlight[key]
		p.mu.Unlock()
		if !staleWarn && !failed && !inFlight {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected stale warning and failed state to clear on successful run")
}
