// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

type staticHealthChecker struct {
	health   models.ServiceHealth
	onCalled func()
}

func (c staticHealthChecker) CheckHealth(_ context.Context, _ string, _ string) (models.ServiceHealth, int) {
	if c.onCalled != nil {
		c.onCalled()
	}
	return c.health, 200
}

type staticServiceRegistry struct {
	checker models.ServiceHealthChecker
}

func (r staticServiceRegistry) CreateService(_ string) models.ServiceHealthChecker {
	return r.checker
}

func waitForLastRun(t *testing.T, p *Poller, key string, timeout time.Duration) time.Time {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		last := p.lastRun[key]
		p.mu.Unlock()
		if !last.IsZero() {
			return last
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected lastRun[%q] within %v", key, timeout)
	return time.Time{}
}

func setupPollerTickTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := tempDir + "/poller_tick_test.db"

	_ = os.Setenv("DASHBRR__DB_TYPE", "sqlite")
	_ = os.Setenv("DASHBRR__DB_PATH", dbPath)

	db, err := database.InitDBWithConfig(database.NewConfig())
	if err != nil {
		t.Fatalf("failed to initialize test DB: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.Unsetenv("DASHBRR__DB_TYPE")
		_ = os.Unsetenv("DASHBRR__DB_PATH")
	}

	return db, cleanup
}

func TestPollerTick_ForcedRunSkipsStatsJobs(t *testing.T) {
	db, cleanup := setupPollerTickTestDB(t)
	defer cleanup()

	err := db.CreateService(context.Background(), &models.ServiceConfiguration{
		InstanceID:  "plex-1",
		DisplayName: "Plex",
		URL:         "http://example",
		APIKey:      "key",
	})
	if err != nil {
		t.Fatalf("failed to seed service: %v", err)
	}

	p := NewPoller(db, NewBroadcaster(sse.NewHub()))
	p.registry = staticServiceRegistry{
		checker: staticHealthChecker{
			health: models.ServiceHealth{Status: "online", Message: "Healthy"},
		},
	}

	statsRan := make(chan struct{}, 1)
	p.jobs["plex"] = []jobSpec{
		{
			name:     "test_stats",
			interval: time.Minute,
			timeout:  time.Second,
			run: func(*Poller, context.Context, models.ServiceConfiguration, string) error {
				statsRan <- struct{}{}
				return nil
			},
		},
	}

	sem := make(chan struct{}, 1)
	p.tick(context.Background(), sem, true, "")

	waitForLastRun(t, p, "plex-1:health", 300*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	select {
	case <-statsRan:
		t.Fatalf("expected forced tick to skip stats jobs")
	default:
	}

	p.mu.Lock()
	if !p.lastRun["plex-1:test_stats"].IsZero() {
		p.mu.Unlock()
		t.Fatalf("expected forced tick to leave stats lastRun empty")
	}
	p.mu.Unlock()
}

func TestPollerTick_HealthCompletesBeforeSlowStatsJob(t *testing.T) {
	p := NewPoller(nil, NewBroadcaster(sse.NewHub()))
	p.registry = staticServiceRegistry{
		checker: staticHealthChecker{
			health: models.ServiceHealth{Status: "online", Message: "Healthy"},
		},
	}

	statsStarted := make(chan struct{}, 1)
	statsDone := make(chan struct{}, 1)
	p.jobs["plex"] = []jobSpec{
		{
			name:     "test_stats",
			interval: time.Minute,
			timeout:  time.Second,
			run: func(*Poller, context.Context, models.ServiceConfiguration, string) error {
				statsStarted <- struct{}{}
				time.Sleep(220 * time.Millisecond)
				statsDone <- struct{}{}
				return nil
			},
		},
	}

	p.mu.Lock()
	p.services = []models.ServiceConfiguration{
		{
			InstanceID: "plex-1",
			URL:        "http://example",
			APIKey:     "key",
		},
	}
	p.loadedAt = time.Now()
	p.mu.Unlock()

	// Single worker slot: verifies queue ordering/priority behavior.
	sem := make(chan struct{}, 1)
	p.tick(context.Background(), sem, false, "")

	healthRanAt := waitForLastRun(t, p, "plex-1:health", 200*time.Millisecond)

	select {
	case <-statsStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected stats job to start")
	}

	// While slow stats is still in progress, health should already be complete.
	p.mu.Lock()
	statsLastRun := p.lastRun["plex-1:test_stats"]
	p.mu.Unlock()
	if !statsLastRun.IsZero() {
		t.Fatalf("expected stats lastRun to remain empty while slow job is running")
	}
	if healthRanAt.IsZero() {
		t.Fatalf("expected health to complete before stats")
	}

	select {
	case <-statsDone:
	case <-time.After(600 * time.Millisecond):
		t.Fatalf("expected stats job to complete")
	}

	waitForLastRun(t, p, "plex-1:test_stats", 200*time.Millisecond)
}
