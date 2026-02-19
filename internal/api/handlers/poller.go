// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/qui"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
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
		"overseerr": {
			{name: "overseerr_requests", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runOverseerrRequests},
		},
		"radarr": {
			{name: "radarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runRadarrQueue},
		},
		"sonarr": {
			{name: "sonarr_queue", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runSonarrQueue},
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
		"maintainerr": {
			{name: "maintainerr_collections", interval: 10 * time.Minute, timeout: pollerLongJobTimeout, run: (*Poller).runMaintainerrCollections},
		},
		"tailscale": {
			{name: "tailscale_devices", interval: 60 * time.Second, timeout: pollerMediumJobTimeout, run: (*Poller).runTailscaleDevices},
		},
		"qui": {
			{name: "qui_overview", interval: 20 * time.Second, timeout: pollerShortJobTimeout, run: (*Poller).runQuiOverview},
		},
	}

	return p
}

func (p *Poller) Start(ctx context.Context) {
	go p.run(ctx)
}

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

	maxJitter := interval / 10
	if maxJitter > pollerMaxJobJitter {
		maxJitter = pollerMaxJobJitter
	}
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
	if serviceType == "general" {
		return true
	}
	return svc.APIKey != ""
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
			Dur("queue_delay", queueDelay).
			Dur("duration", duration)

		switch {
		case err != nil:
			failedLog := log.Warn().
				Err(err).
				Str("instance", svc.InstanceID).
				Str("service", serviceType).
				Str("job", job).
				Dur("queue_delay", queueDelay).
				Dur("duration", duration)
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
				Dur("queue_delay", queueDelay).
				Dur("duration", duration).
				Msg("poller job exceeded timeout")
		case duration >= pollerSlowJobThreshold:
			log.Warn().
				Str("instance", svc.InstanceID).
				Str("service", serviceType).
				Str("job", job).
				Dur("queue_delay", queueDelay).
				Dur("duration", duration).
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

func summarizeRadarrQueue(records []types.RadarrQueueRecord) (int, int64) {
	downloading := 0
	var totalSize int64

	for _, record := range records {
		totalSize += record.Size
		if record.Status == "downloading" {
			downloading++
		}
	}

	return downloading, totalSize
}

func summarizeSonarrQueue(records []types.QueueRecord) (int, int, int64) {
	downloading := 0
	episodeCount := 0
	var totalSize int64

	for _, record := range records {
		totalSize += record.Size
		if record.Status == "downloading" {
			downloading++
		}
		episodeCount += len(record.Episodes)
	}

	return downloading, episodeCount, totalSize
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

func (p *Poller) runRadarrQueue(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &radarr.RadarrService{}
	records, err := service.GetQueueForHealth(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}
	if records == nil {
		records = []types.RadarrQueueRecord{}
	}

	resp := &types.RadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	publishInternalServiceUpdate(p.bc, buildRadarrQueueServiceUpdate(svc.InstanceID, resp))
	return nil
}

func (p *Poller) runSonarrQueue(ctx context.Context, svc models.ServiceConfiguration, _ string) error {
	service := &sonarr.SonarrService{}
	records, err := service.GetQueueForHealth(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return err
	}
	if records == nil {
		records = []types.QueueRecord{}
	}

	resp := &types.SonarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	publishInternalServiceUpdate(p.bc, buildSonarrQueueServiceUpdate(svc.InstanceID, resp))
	return nil
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
