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

func waitForLastOKRun(t *testing.T, p *Poller, key string, timeout time.Duration) time.Time {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		last := p.lastOKRun[key]
		p.mu.Unlock()
		if !last.IsZero() {
			return last
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected lastOKRun[%q] within %v", key, timeout)
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

func TestPollerTick_ForcedRunBootstrapsStatsOnce(t *testing.T) {
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

	healthSem := make(chan struct{}, 1)
	statsSem := make(chan struct{}, 1)
	p.tick(context.Background(), healthSem, statsSem, true, "")

	waitForLastRun(t, p, "plex-1:health", 300*time.Millisecond)
	waitForLastRun(t, p, "plex-1:test_stats", 300*time.Millisecond)
	waitForLastOKRun(t, p, "plex-1:test_stats", 300*time.Millisecond)

	select {
	case <-statsRan:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected forced startup tick to bootstrap stats jobs")
	}

	p.tick(context.Background(), healthSem, statsSem, true, "")

	time.Sleep(120 * time.Millisecond)
	select {
	case <-statsRan:
		t.Fatalf("expected forced startup tick to skip already-bootstrapped stats jobs")
	default:
	}
}

func TestPollerTick_HealthStillRunsWithSlowStatsJob(t *testing.T) {
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
				time.Sleep(450 * time.Millisecond)
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
	healthSem := make(chan struct{}, 1)
	statsSem := make(chan struct{}, 1)
	p.tick(context.Background(), healthSem, statsSem, false, "")

	waitForLastRun(t, p, "plex-1:health", time.Second)

	select {
	case <-statsStarted:
	case <-time.After(time.Second):
		t.Fatalf("expected stats job to start")
	}

	select {
	case <-statsDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected stats job to complete")
	}

	waitForLastRun(t, p, "plex-1:test_stats", time.Second)
}

func TestPollerTick_HealthNotBlockedBySaturatedStatsSemaphore(t *testing.T) {
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

	healthSem := make(chan struct{}, 1)
	statsSem := make(chan struct{}, 1)
	statsSem <- struct{}{} // block stats lane

	p.tick(context.Background(), healthSem, statsSem, false, "")

	waitForLastRun(t, p, "plex-1:health", 300*time.Millisecond)

	select {
	case <-statsRan:
		t.Fatalf("expected stats to stay blocked while stats semaphore is saturated")
	case <-time.After(100 * time.Millisecond):
	}

	<-statsSem // unblock stats lane

	select {
	case <-statsRan:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected stats to run after stats semaphore is released")
	}

	waitForLastRun(t, p, "plex-1:test_stats", 300*time.Millisecond)
}

func TestPollerTick_ForcedRefreshOnlyTargetsRequestedInstance(t *testing.T) {
	db, cleanup := setupPollerTickTestDB(t)
	defer cleanup()

	err := db.CreateService(context.Background(), &models.ServiceConfiguration{
		InstanceID:  "plex-1",
		DisplayName: "Plex",
		URL:         "http://plex.example",
		APIKey:      "key",
	})
	if err != nil {
		t.Fatalf("failed to seed plex service: %v", err)
	}

	err = db.CreateService(context.Background(), &models.ServiceConfiguration{
		InstanceID:  "radarr-1",
		DisplayName: "Radarr",
		URL:         "http://radarr.example",
		APIKey:      "key",
	})
	if err != nil {
		t.Fatalf("failed to seed radarr service: %v", err)
	}

	p := NewPoller(db, NewBroadcaster(sse.NewHub()))
	p.registry = staticServiceRegistry{
		checker: staticHealthChecker{
			health: models.ServiceHealth{Status: "online", Message: "Healthy"},
		},
	}
	radarrStatsRan := make(chan struct{}, 2)
	plexStatsRan := make(chan struct{}, 2)
	p.jobs["radarr"] = []jobSpec{
		{
			name:     "test_radarr_stats",
			interval: time.Minute,
			timeout:  time.Second,
			run: func(*Poller, context.Context, models.ServiceConfiguration, string) error {
				radarrStatsRan <- struct{}{}
				return nil
			},
		},
	}
	p.jobs["plex"] = []jobSpec{
		{
			name:     "test_plex_stats",
			interval: time.Minute,
			timeout:  time.Second,
			run: func(*Poller, context.Context, models.ServiceConfiguration, string) error {
				plexStatsRan <- struct{}{}
				return nil
			},
		},
	}

	healthSem := make(chan struct{}, 1)
	statsSem := make(chan struct{}, 1)
	p.tick(context.Background(), healthSem, statsSem, true, "radarr-1")

	waitForLastRun(t, p, "radarr-1:health", 300*time.Millisecond)
	waitForLastRun(t, p, "radarr-1:test_radarr_stats", 300*time.Millisecond)

	select {
	case <-radarrStatsRan:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected forced refresh for radarr-1 to run radarr stats")
	}

	time.Sleep(60 * time.Millisecond)
	p.mu.Lock()
	plexHealthRun := p.lastRun["plex-1:health"]
	plexStatsRun := p.lastRun["plex-1:test_plex_stats"]
	p.mu.Unlock()
	if !plexHealthRun.IsZero() {
		t.Fatalf("expected forced refresh for radarr-1 to skip unrelated plex-1 health run")
	}
	if !plexStatsRun.IsZero() {
		t.Fatalf("expected forced refresh for radarr-1 to skip unrelated plex-1 stats run")
	}
	select {
	case <-plexStatsRan:
		t.Fatalf("expected no plex stats run for radarr-targeted forced refresh")
	default:
	}
}
