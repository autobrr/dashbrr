// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/bazarr"
	"github.com/autobrr/dashbrr/internal/services/jellyfin"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/nzbget"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/qui"
	"github.com/autobrr/dashbrr/internal/services/sabnzbd"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/services/traefik"
	"github.com/autobrr/dashbrr/internal/services/uptimekuma"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	pollerTickInterval      = 1 * time.Second
	pollerServiceReloadTTL  = 15 * time.Second
	pollerHealthTimeout     = 25 * time.Second
	pollerPendingTimeout    = 5 * time.Second
	pollerDefaultJobTimeout = 25 * time.Second
	pollerLongJobTimeout    = 35 * time.Second
	pollerMaxConcurrentUpst = 8
	pollerMaxConcurrentHlt  = 16
	pollerSlowJobThreshold  = 5 * time.Second
	pollerFailedRetryDelay  = 10 * time.Second
	pollerMaxJobJitter      = 5 * time.Second
	pollerMinStaleThreshold = 30 * time.Second
	pollerMaxStaleThreshold = 10 * time.Minute
	pollerShortJobTimeout   = 12 * time.Second
	pollerMediumJobTimeout  = 20 * time.Second
)

func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

type jobRunner func(*Poller, context.Context, models.ServiceConfiguration, string) error

type jobSpec struct {
	name     string
	interval time.Duration
	timeout  time.Duration
	run      jobRunner
}

type Poller struct {
	db       *database.DB
	bc       *Broadcaster
	registry models.ServiceCreator
	jobs     map[string][]jobSpec

	mu        sync.Mutex
	lastRun   map[string]time.Time // key: instanceId + ":" + job (last attempt)
	lastOKRun map[string]time.Time // key: instanceId + ":" + job (last successful attempt)
	failed    map[string]bool
	staleWarn map[string]bool
	inFlight  map[string]bool
	services  []models.ServiceConfiguration
	loadedAt  time.Time
	startedAt time.Time

	// trigger refresh now
	refreshCh chan refreshReq
	// first health snapshot observability
	firstHealthSeen map[string]bool
}

type refreshReq struct {
	instanceID string
}

func NewPoller(db *database.DB, bc *Broadcaster) *Poller {
	p := &Poller{
		db:              db,
		bc:              bc,
		registry:        models.NewServiceRegistry(),
		lastRun:         make(map[string]time.Time),
		lastOKRun:       make(map[string]time.Time),
		failed:          make(map[string]bool),
		staleWarn:       make(map[string]bool),
		inFlight:        make(map[string]bool),
		refreshCh:       make(chan refreshReq, 64),
		firstHealthSeen: make(map[string]bool),
	}

	p.jobs = map[string][]jobSpec{
		"plex": {
			{name: "plex_sessions", interval: 10 * time.Second, timeout: pollerShortJobTimeout, run: (*Poller).runPlexSessions},
		},
		"jellyfin": {
			{name: "jellyfin_summary", interval: 10 * time.Second, timeout: pollerShortJobTimeout, run: (*Poller).runJellyfinSummary},
		},
		"uptimekuma": {
			{name: "uptimekuma_summary", interval: 30 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runUptimeKumaSummary},
		},
		"overseerr": {
			{name: "overseerr_requests", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runOverseerrRequests},
		},
		"radarr": {
			{name: "radarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: arrQueueJob(arr.Radarr)},
		},
		"lidarr": {
			{name: "lidarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: arrQueueJob(arr.Lidarr)},
		},
		"readarr": {
			{name: "readarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: arrQueueJob(arr.Readarr)},
		},
		"sonarr": {
			{name: "sonarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: arrQueueJob(arr.Sonarr)},
		},
		"whisparr": {
			{name: "whisparr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: arrQueueJob(arr.Whisparr)},
		},
		"prowlarr": {
			{name: "prowlarr_stats", interval: 120 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runProwlarrStats},
			{name: "prowlarr_indexers", interval: 120 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runProwlarrIndexers},
		},
		"autobrr": {
			{name: "autobrr_stats", interval: 120 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runAutobrrStats},
			{name: "autobrr_irc_status", interval: 120 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runAutobrrIRC},
			{name: "autobrr_releases", interval: 120 * time.Second, timeout: pollerLongJobTimeout, run: (*Poller).runAutobrrReleases},
		},
		"bazarr": {
			{name: "bazarr_summary", interval: 90 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runBazarrSummary},
		},
		"sabnzbd": {
			{name: "sabnzbd_summary", interval: 45 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runSabnzbdSummary},
		},
		"nzbget": {
			{name: "nzbget_summary", interval: 45 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runNzbgetSummary},
		},
		"maintainerr": {
			{name: "maintainerr_collections", interval: 10 * time.Minute, timeout: pollerLongJobTimeout, run: (*Poller).runMaintainerrCollections},
		},
		"tailscale": {
			{name: "tailscale_devices", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runTailscaleDevices},
		},
		"qui": {
			{name: "qui_overview", interval: 20 * time.Second, timeout: pollerShortJobTimeout, run: (*Poller).runQuiOverview},
		},
		"traefik": {
			{name: "traefik_summary", interval: 30 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runTraefikSummary},
		},
	}

	return p
}

func (p *Poller) Start(ctx context.Context) {
	go p.run(ctx)
}

// Refresh asks the poller to run all jobs for one instance now.
//
// ponytail: maybeRun skips a forced run while the same detail job is in flight, so
// the running job can publish data from before the user action. The stale item stays
// until the next scheduled run of that job: one interval plus up to 5s of jitter
// (about 65s for queues). If users report it, mark the job dirty and run it again
// when the current run finishes.
func (p *Poller) Refresh(instanceID string) {
	select {
	case p.refreshCh <- refreshReq{instanceID: instanceID}:
	default:
	}
}

func (p *Poller) run(ctx context.Context) {
	p.mu.Lock()
	p.startedAt = time.Now()
	p.firstHealthSeen = make(map[string]bool)
	p.mu.Unlock()

	log.Info().Msg("poller started")
	defer log.Info().Msg("poller stopped")

	healthSem := make(chan struct{}, pollerMaxConcurrentHlt) // keep health responsive
	statsSem := make(chan struct{}, pollerMaxConcurrentUpst) // cap stats/detail concurrency

	// small tick; jobs self-throttle by lastRun
	t := time.NewTicker(pollerTickInterval)
	defer t.Stop()

	// initial blast
	p.tick(ctx, healthSem, statsSem, true, "")

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-p.refreshCh:
			p.tick(ctx, healthSem, statsSem, true, req.instanceID)
		case <-t.C:
			p.tick(ctx, healthSem, statsSem, false, "")
		}
	}
}

func (p *Poller) tick(ctx context.Context, healthSem, statsSem chan struct{}, force bool, onlyInstance string) {
	services := p.getServices(ctx, force || onlyInstance != "")
	if services == nil {
		return
	}

	type pollerService struct {
		cfg        models.ServiceConfiguration
		kind       string
		configured bool
	}

	pollServices := make([]pollerService, 0, len(services))

	for _, svc := range services {
		if onlyInstance != "" && svc.InstanceID != onlyInstance {
			continue
		}

		serviceType, ok := models.ServiceTypeFromInstanceID(svc.InstanceID)
		if !ok {
			continue
		}
		pollServices = append(pollServices, pollerService{
			cfg:        svc,
			kind:       serviceType,
			configured: isServiceConfigured(serviceType, svc),
		})
	}
	sort.Slice(pollServices, func(i, j int) bool {
		return pollServices[i].cfg.InstanceID < pollServices[j].cfg.InstanceID
	})

	// Pass 1: enqueue health for every service first so version-bearing health checks
	// are not delayed behind stats jobs on startup and forced refreshes.
	for _, ps := range pollServices {
		if ps.configured {
			p.maybeRun(ctx, healthSem, ps.cfg, ps.kind, "health", 30*time.Second, pollerHealthTimeout, force, (*Poller).runHealth)
			continue
		}
		p.maybeRun(ctx, healthSem, ps.cfg, ps.kind, "health", 60*time.Second, pollerPendingTimeout, force, (*Poller).runPending)
	}

	// Pass 2: enqueue detail jobs for configured services.
	// Forced tick behavior:
	// - startup/global forced run: only bootstrap jobs without a successful prior run
	// - targeted forced run: run all detail jobs for the target instance immediately
	for _, ps := range pollServices {
		if !ps.configured {
			continue
		}
		for _, job := range p.jobs[ps.kind] {
			if force {
				if onlyInstance == "" && !p.shouldRunBootstrapDetail(ps.cfg.InstanceID, job.name) {
					continue
				}
			}
			p.maybeRun(ctx, statsSem, ps.cfg, ps.kind, job.name, job.interval, effectiveJobTimeout(job.timeout), force, job.run)
		}
	}
}

func (p *Poller) shouldRunBootstrapDetail(instanceID, job string) bool {
	key := instanceID + ":" + job

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastOKRun[key].IsZero()
}

func effectiveJobTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return pollerDefaultJobTimeout
	}
	return timeout
}

func pollerStaleDataThreshold(interval time.Duration) time.Duration {
	threshold := interval * 2
	if threshold < pollerMinStaleThreshold {
		return pollerMinStaleThreshold
	}
	if threshold > pollerMaxStaleThreshold {
		return pollerMaxStaleThreshold
	}
	return threshold
}

func applyPollerJobJitter(key string, interval time.Duration) time.Duration {
	if interval <= pollerTickInterval {
		return interval
	}

	maxJitter := min(interval/10, pollerMaxJobJitter)
	if maxJitter <= 0 {
		return interval
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	jitter := time.Duration(hasher.Sum32() % uint32(maxJitter+1))

	return interval + jitter
}

func isServiceConfigured(serviceType string, svc models.ServiceConfiguration) bool {
	if svc.URL == "" {
		return false
	}
	if !serviceRequiresAPIKey(serviceType) {
		return true
	}
	if serviceAllowsURLCredentials(serviceType) && svc.APIKey == "" {
		return urlHasUserCredentials(svc.URL)
	}
	return svc.APIKey != ""
}

func urlHasUserCredentials(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.User == nil {
		return false
	}
	username := strings.TrimSpace(u.User.Username())
	password, _ := u.User.Password()
	return username != "" && strings.TrimSpace(password) != ""
}

func (p *Poller) getServices(ctx context.Context, force bool) []models.ServiceConfiguration {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reload config periodically or when forced.
	if !force && time.Since(p.loadedAt) < pollerServiceReloadTTL && p.services != nil {
		return p.services
	}

	services, err := p.db.GetAllServices(ctx)
	if err != nil {
		log.Error().Err(err).Msg("poller: failed to load services")
		return nil
	}

	p.services = services
	p.loadedAt = time.Now()
	return services
}

func (p *Poller) maybeRun(ctx context.Context, sem chan struct{}, svc models.ServiceConfiguration, serviceType string, job string, interval time.Duration, timeout time.Duration, force bool, run jobRunner) {
	key := svc.InstanceID + ":" + job

	p.mu.Lock()
	if p.inFlight[key] {
		p.mu.Unlock()
		return
	}
	last := p.lastRun[key]
	currentInterval := interval
	if p.failed[key] {
		currentInterval = pollerFailedRetryDelay
	} else if !force && !last.IsZero() {
		currentInterval = applyPollerJobJitter(key, interval)
	}
	due := force || last.IsZero() || time.Since(last) >= currentInterval
	if !due {
		p.mu.Unlock()
		return
	}
	p.inFlight[key] = true
	p.mu.Unlock()

	go func() {
		queuedAt := time.Now()

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			p.mu.Lock()
			delete(p.inFlight, key)
			p.mu.Unlock()
			return
		}
		defer func() { <-sem }()
		defer func() {
			p.mu.Lock()
			delete(p.inFlight, key)
			p.mu.Unlock()
		}()

		jobCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		started := time.Now()
		queueDelay := started.Sub(queuedAt)

		p.mu.Lock()
		p.lastRun[key] = started
		p.mu.Unlock()

		var err error
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("panic: %v", recovered)
				}
			}()
			err = run(p, jobCtx, svc, serviceType)
		}()
		duration := time.Since(started)
		now := time.Now()

		var (
			lastOKRun       time.Time
			shouldWarnStale bool
			staleFor        time.Duration
			staleThreshold  time.Duration
		)

		p.mu.Lock()
		if err != nil {
			p.failed[key] = true
			lastOKRun = p.lastOKRun[key]
			if !lastOKRun.IsZero() {
				staleFor = now.Sub(lastOKRun)
				staleThreshold = pollerStaleDataThreshold(interval)
				if staleFor >= staleThreshold && !p.staleWarn[key] {
					p.staleWarn[key] = true
					shouldWarnStale = true
				}
			}
		} else {
			p.failed[key] = false
			p.lastOKRun[key] = now
			p.staleWarn[key] = false
		}
		p.mu.Unlock()

		baseLog := log.Trace().
			Str("instance", svc.InstanceID).
			Str("service", serviceType).
			Str("job", job).
			Float64("queue_delay_ms", durationMs(queueDelay)).
			Float64("duration_ms", durationMs(duration))

		switch {
		case err != nil:
			failedLog := log.Warn().
				Err(err).
				Str("instance", svc.InstanceID).
				Str("service", serviceType).
				Str("job", job).
				Float64("queue_delay_ms", durationMs(queueDelay)).
				Float64("duration_ms", durationMs(duration))
			if !lastOKRun.IsZero() {
				failedLog = failedLog.Dur("stale_for", staleFor)
			}
			failedLog.Msg("poller job failed")
			if job != "health" && !lastOKRun.IsZero() {
				if p.bc.PublishLatest(svc.InstanceID) {
					log.Debug().
						Str("instance", svc.InstanceID).
						Str("service", serviceType).
						Str("job", job).
						Msg("republished last-known service payload after job failure")
				}
			}
			if shouldWarnStale {
				log.Warn().
					Str("instance", svc.InstanceID).
					Str("service", serviceType).
					Str("job", job).
					Dur("stale_for", staleFor).
					Dur("stale_threshold", staleThreshold).
					Msg("poller job data is stale")
			}
		case jobCtx.Err() == context.DeadlineExceeded:
			log.Warn().
				Str("instance", svc.InstanceID).
				Str("service", serviceType).
				Str("job", job).
				Float64("queue_delay_ms", durationMs(queueDelay)).
				Float64("duration_ms", durationMs(duration)).
				Msg("poller job exceeded timeout")
		case duration >= pollerSlowJobThreshold:
			log.Warn().
				Str("instance", svc.InstanceID).
				Str("service", serviceType).
				Str("job", job).
				Float64("queue_delay_ms", durationMs(queueDelay)).
				Float64("duration_ms", durationMs(duration)).
				Msg("poller job completed slowly")
		default:
			baseLog.Msg("poller job completed")
		}
	}()
}

func (p *Poller) runHealth(ctx context.Context, svc models.ServiceConfiguration, serviceType string) error {
	checker := p.registry.CreateService(serviceType)
	if checker == nil {
		publishHealthServiceUpdate(p.bc, models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "error",
			Message:     "Unsupported service type: " + serviceType,
			LastChecked: time.Now(),
		})
		return nil
	}

	health, _ := checker.CheckHealth(ctx, svc.URL, svc.APIKey)
	health.ServiceID = svc.InstanceID
	if health.LastChecked.IsZero() {
		health.LastChecked = time.Now()
	}
	publishHealthServiceUpdate(p.bc, health)
	p.logFirstHealthSeen(svc.InstanceID, serviceType, health.Status)
	return nil
}

func (p *Poller) runPending(_ context.Context, svc models.ServiceConfiguration, serviceType string) error {
	publishHealthServiceUpdate(p.bc, models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "pending",
		Message:     "Service not configured",
		LastChecked: time.Now(),
	})
	p.logFirstHealthSeen(svc.InstanceID, serviceType, "pending")
	return nil
}

func (p *Poller) markFirstHealthSeen(instanceID string) (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.firstHealthSeen[instanceID] {
		return 0, false
	}
	if p.startedAt.IsZero() {
		return 0, false
	}

	p.firstHealthSeen[instanceID] = true
	return time.Since(p.startedAt), true
}

func (p *Poller) logFirstHealthSeen(instanceID, serviceType, status string) {
	elapsed, shouldLog := p.markFirstHealthSeen(instanceID)
	if !shouldLog {
		return
	}

	log.Info().
		Str("instance", instanceID).
		Str("service", serviceType).
		Str("status", status).
		Dur("startup_elapsed", elapsed).
		Msg("poller first health seen")
}

func countTranscodingSessions(sessions []types.PlexSession) int {
	transcoding := 0
	for _, session := range sessions {
		if session.TranscodeSession != nil {
			transcoding++
		}
	}

	return transcoding
}

func countOnlineDevices(devices []tailscale.Device) int {
	online := 0
	for _, device := range devices {
		if device.Online {
			online++
		}
	}

	return online
}

func summarizeQuiCardStatus(summary types.QuiTransferSummary) string {
	if summary.TotalInstances == 0 {
		return "warning"
	}
	if summary.ActiveInstances == 0 {
		return "warning"
	}
	if summary.ConnectedInstances < summary.ActiveInstances {
		return "warning"
	}
	return "online"
}

func (p *Poller) runPlexSessions(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &plex.PlexService{}
	sessions, err := service.GetSessions(ctx, svc.URL, svc.APIKey)
	if err != nil || sessions == nil {
		if err != nil {
			return err
		}
		return nil
	}

	metadata := sessions.MediaContainer.Metadata
	if metadata == nil {
		metadata = []types.PlexSession{}
	}

	publishInternalServiceUpdate(p.bc, buildPlexSessionsServiceUpdate(svc.InstanceID, metadata))
	return nil
}

func (p *Poller) runJellyfinSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := jellyfin.NewJellyfinService().(*jellyfin.JellyfinService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildJellyfinSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}

func (p *Poller) runUptimeKumaSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := uptimekuma.NewUptimeKumaService().(*uptimekuma.UptimeKumaService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildUptimeKumaSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}

func (p *Poller) runOverseerrRequests(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &overseerr.OverseerrService{}
	service.SetDB(p.db)

	stats, err := service.GetRequests(ctx, svc.URL, svc.APIKey)
	if err != nil || stats == nil {
		if err != nil {
			return err
		}
		return nil
	}

	if stats.Requests == nil {
		stats.Requests = []types.MediaRequest{}
	}

	publishInternalServiceUpdate(p.bc, buildOverseerrRequestsServiceUpdate(svc.InstanceID, stats))
	return nil
}

// arrQueueJob returns the queue job of one *arr app.
func arrQueueJob(app arr.App) jobRunner {
	return func(p *Poller, ctx context.Context, svc models.ServiceConfiguration, _ string) error {
		page, err := app.FetchQueue(ctx, svc.URL, svc.APIKey)
		if err != nil {
			return err
		}

		publishInternalServiceUpdate(p.bc, buildArrQueueServiceUpdate(app, svc.InstanceID, page))
		return nil
	}
}

func (p *Poller) runProwlarrStats(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	ps := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)

	idxStats, err := ps.GetIndexerStats(ctx, svc.URL, svc.APIKey)
	if err != nil || idxStats == nil {
		if err != nil {
			return err
		}
		return nil
	}

	totalGrabs := 0
	totalFails := 0
	for _, stat := range idxStats.Indexers {
		totalGrabs += stat.NumberOfGrabs
		totalFails += stat.NumberOfFailedGrabs
	}

	publishInternalServiceUpdate(p.bc, buildProwlarrStatsServiceUpdate(svc.InstanceID, types.ProwlarrStatsResponse{
		GrabCount:    totalGrabs,
		FailCount:    totalFails,
		IndexerCount: len(idxStats.Indexers),
	}))
	return nil
}

func (p *Poller) runProwlarrIndexers(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	ps := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)

	indexers, err := ps.GetIndexers(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildProwlarrIndexersServiceUpdate(svc.InstanceID, indexers))
	return nil
}

func (p *Poller) runAutobrrStats(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &autobrr.AutobrrService{}

	stats, err := service.GetReleaseStats(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildAutobrrStatsServiceUpdate(svc.InstanceID, stats))
	return nil
}

func (p *Poller) runAutobrrIRC(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &autobrr.AutobrrService{}

	irc, err := service.GetIRCStatus(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	health, eventType := buildAutobrrIRCServiceUpdate(svc.InstanceID, irc)
	if eventType == models.ServiceEventInternal {
		publishInternalServiceUpdate(p.bc, health)
		return nil
	}
	publishHealthServiceUpdate(p.bc, health)
	return nil
}

func (p *Poller) runAutobrrReleases(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &autobrr.AutobrrService{}

	releases, err := service.GetReleases(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildAutobrrReleasesServiceUpdate(svc.InstanceID, releases))
	return nil
}

func (p *Poller) runBazarrSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := bazarr.NewBazarrService().(*bazarr.BazarrService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildBazarrSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}

func (p *Poller) runSabnzbdSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := sabnzbd.NewSabnzbdService().(*sabnzbd.SabnzbdService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildSabnzbdSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}

func (p *Poller) runNzbgetSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := nzbget.NewNzbgetService().(*nzbget.NzbgetService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildNzbgetSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}

func (p *Poller) runMaintainerrCollections(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &maintainerr.MaintainerrService{}
	collections, err := service.GetCollections(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}
	if collections == nil {
		collections = []maintainerr.Collection{}
	}

	publishInternalServiceUpdate(p.bc, buildMaintainerrCollectionsServiceUpdate(svc.InstanceID, collections))
	return nil
}

func (p *Poller) runTailscaleDevices(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &tailscale.TailscaleService{}
	devices, err := service.GetDevices(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}
	if devices == nil {
		devices = []tailscale.Device{}
	}

	publishInternalServiceUpdate(p.bc, buildTailscaleDevicesServiceUpdate(svc.InstanceID, devices))
	return nil
}

func (p *Poller) runQuiOverview(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := qui.NewQuiService().(*qui.QuiService)

	instances, err := service.GetInstances(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}
	if instances == nil {
		instances = []types.QuiInstance{}
	}

	summary, transfers := service.GetAggregatedTransferInfo(ctx, svc.URL, svc.APIKey, instances)

	publishInternalServiceUpdate(p.bc, buildQuiOverviewServiceUpdate(svc.InstanceID, instances, summary, transfers))
	return nil
}

func (p *Poller) runTraefikSummary(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := traefik.NewTraefikService().(*traefik.TraefikService)
	summary, err := service.GetSummary(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}

	publishInternalServiceUpdate(p.bc, buildTraefikSummaryServiceUpdate(svc.InstanceID, &summary))
	return nil
}
