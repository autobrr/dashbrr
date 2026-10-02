// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/sse"
)

func TestNewPoller_AutobrrJobsAreSplit(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["autobrr"]

	if len(jobs) != 3 {
		t.Fatalf("autobrr job count = %d, want 3", len(jobs))
	}

	want := []string{"autobrr_stats", "autobrr_irc_status", "autobrr_releases"}
	for i, name := range want {
		if jobs[i].name != name {
			t.Fatalf("autobrr job[%d] = %q, want %q", i, jobs[i].name, name)
		}
	}
}

func TestNewPoller_ProwlarrJobsAreSplit(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["prowlarr"]

	if len(jobs) != 2 {
		t.Fatalf("prowlarr job count = %d, want 2", len(jobs))
	}

	want := []string{"prowlarr_stats", "prowlarr_indexers"}
	for i, name := range want {
		if jobs[i].name != name {
			t.Fatalf("prowlarr job[%d] = %q, want %q", i, jobs[i].name, name)
		}
	}
}

func TestNewPoller_QuiJobsAreOverviewOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["qui"]

	if len(jobs) != 1 {
		t.Fatalf("qui job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "qui_overview" {
		t.Fatalf("qui job[0] = %q, want %q", jobs[0].name, "qui_overview")
	}
}

func TestNewPoller_TraefikJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["traefik"]

	if len(jobs) != 1 {
		t.Fatalf("traefik job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "traefik_summary" {
		t.Fatalf("traefik job[0] = %q, want %q", jobs[0].name, "traefik_summary")
	}
}

func TestArrQueueJobs_PublishQueuePage(t *testing.T) {
	t.Parallel()

	for _, app := range arr.Apps {
		t.Run(app.Name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/"+app.APIVersion+"/queue" {
					t.Errorf("path = %q", r.URL.Path)
				}
				_, _ = w.Write([]byte(`{"totalRecords":37,"records":[{"id":1,"title":"one"}]}`))
			}))
			t.Cleanup(srv.Close)

			bc := NewBroadcaster(sse.NewHub())
			p := NewPoller(nil, bc)
			jobs := p.jobs[app.Name]
			if len(jobs) != 1 || jobs[0].name != app.Name+"_queue" {
				t.Fatalf("jobs = %+v, want one %s_queue job", jobs, app.Name)
			}

			instanceID := app.Name + "-1"
			svc := models.ServiceConfiguration{InstanceID: instanceID, URL: srv.URL, APIKey: "key"}
			if err := jobs[0].run(p, t.Context(), svc, ""); err != nil {
				t.Fatalf("run: %v", err)
			}

			health := bc.latest[instanceID]
			if health.Message != app.Name+"_queue" {
				t.Fatalf("Message = %q, want %s_queue", health.Message, app.Name)
			}
			stats, ok := health.Stats[app.Name].(map[string]any)
			if !ok {
				t.Fatalf("Stats[%q] missing: %+v", app.Name, health.Stats)
			}
			page, ok := stats["queue"].(arr.QueuePage)
			if !ok {
				t.Fatalf("queue = %T, want arr.QueuePage", stats["queue"])
			}
			if page.TotalRecords != 37 || len(page.Records) != 1 {
				t.Fatalf("queue = %+v, want totalRecords 37 with 1 record", page)
			}
		})
	}
}

func TestNewPoller_BazarrJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["bazarr"]

	if len(jobs) != 1 {
		t.Fatalf("bazarr job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "bazarr_summary" {
		t.Fatalf("bazarr job[0] = %q, want %q", jobs[0].name, "bazarr_summary")
	}
}

func TestNewPoller_SabnzbdJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["sabnzbd"]

	if len(jobs) != 1 {
		t.Fatalf("sabnzbd job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "sabnzbd_summary" {
		t.Fatalf("sabnzbd job[0] = %q, want %q", jobs[0].name, "sabnzbd_summary")
	}
}

func TestNewPoller_NzbgetJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["nzbget"]

	if len(jobs) != 1 {
		t.Fatalf("nzbget job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "nzbget_summary" {
		t.Fatalf("nzbget job[0] = %q, want %q", jobs[0].name, "nzbget_summary")
	}
}

func TestNewPoller_JellyfinJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["jellyfin"]

	if len(jobs) != 1 {
		t.Fatalf("jellyfin job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "jellyfin_summary" {
		t.Fatalf("jellyfin job[0] = %q, want %q", jobs[0].name, "jellyfin_summary")
	}
}

func TestNewPoller_UptimeKumaJobsAreSummaryOnly(t *testing.T) {
	t.Parallel()

	poller := NewPoller(nil, nil)
	jobs := poller.jobs["uptimekuma"]

	if len(jobs) != 1 {
		t.Fatalf("uptimekuma job count = %d, want 1", len(jobs))
	}

	if jobs[0].name != "uptimekuma_summary" {
		t.Fatalf("uptimekuma job[0] = %q, want %q", jobs[0].name, "uptimekuma_summary")
	}
}

func TestEffectiveJobTimeout(t *testing.T) {
	t.Parallel()

	if got := effectiveJobTimeout(0); got != pollerDefaultJobTimeout {
		t.Fatalf("effectiveJobTimeout(0) = %v, want %v", got, pollerDefaultJobTimeout)
	}

	override := 7 * time.Second
	if got := effectiveJobTimeout(override); got != override {
		t.Fatalf("effectiveJobTimeout(override) = %v, want %v", got, override)
	}
}

func TestPollerStaleDataThreshold(t *testing.T) {
	t.Parallel()

	if got := pollerStaleDataThreshold(5 * time.Second); got != pollerMinStaleThreshold {
		t.Fatalf("pollerStaleDataThreshold(5s) = %v, want %v", got, pollerMinStaleThreshold)
	}

	if got := pollerStaleDataThreshold(40 * time.Second); got != 80*time.Second {
		t.Fatalf("pollerStaleDataThreshold(40s) = %v, want %v", got, 80*time.Second)
	}

	if got := pollerStaleDataThreshold(8 * time.Minute); got != pollerMaxStaleThreshold {
		t.Fatalf("pollerStaleDataThreshold(8m) = %v, want %v", got, pollerMaxStaleThreshold)
	}
}

func TestApplyPollerJobJitter(t *testing.T) {
	t.Parallel()

	base := 60 * time.Second
	a := applyPollerJobJitter("sonarr-1:sonarr_queue", base)
	b := applyPollerJobJitter("sonarr-1:sonarr_queue", base)

	if a != b {
		t.Fatalf("jitter should be deterministic, got %v and %v", a, b)
	}
	if a < base {
		t.Fatalf("jittered interval should not be below base: got %v base %v", a, base)
	}
	if a > base+pollerMaxJobJitter {
		t.Fatalf("jittered interval should be bounded: got %v max %v", a, base+pollerMaxJobJitter)
	}
}
