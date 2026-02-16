// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/types"
)

type RefreshKind string

const (
	RefreshHealth RefreshKind = "health"
	RefreshStats  RefreshKind = "stats"
	RefreshAll    RefreshKind = "all"
)

const (
	pollerTickInterval      = 1 * time.Second
	pollerServiceReloadTTL  = 15 * time.Second
	pollerJobTimeout        = 25 * time.Second
	pollerMaxConcurrentUpst = 4
)

type jobRunner func(*Poller, context.Context, models.ServiceConfiguration, string)

type jobSpec struct {
	name     string
	interval time.Duration
	run      jobRunner
}

type Poller struct {
	db       *database.DB
	bc       *Broadcaster
	registry models.ServiceCreator
	jobs     map[string][]jobSpec

	mu       sync.Mutex
	lastRun  map[string]time.Time // key: instanceId + ":" + job
	inFlight map[string]bool
	services []models.ServiceConfiguration
	loadedAt time.Time

	// trigger refresh now
	refreshCh chan refreshReq
}

type refreshReq struct {
	instanceID string
	kind       RefreshKind
}

func NewPoller(db *database.DB, bc *Broadcaster) *Poller {
	p := &Poller{
		db:        db,
		bc:        bc,
		registry:  models.NewServiceRegistry(),
		lastRun:   make(map[string]time.Time),
		inFlight:  make(map[string]bool),
		refreshCh: make(chan refreshReq, 64),
	}

	p.jobs = map[string][]jobSpec{
		"plex": {
			{name: "plex_sessions", interval: 10 * time.Second, run: (*Poller).runPlexSessions},
		},
		"overseerr": {
			{name: "overseerr_requests", interval: 60 * time.Second, run: (*Poller).runOverseerrRequests},
		},
		"radarr": {
			{name: "radarr_queue", interval: 60 * time.Second, run: (*Poller).runRadarrQueue},
		},
		"sonarr": {
			{name: "sonarr_queue", interval: 60 * time.Second, run: (*Poller).runSonarrQueue},
		},
		"prowlarr": {
			{name: "prowlarr", interval: 120 * time.Second, run: (*Poller).runProwlarr},
		},
		"autobrr": {
			{name: "autobrr", interval: 120 * time.Second, run: (*Poller).runAutobrr},
		},
		"maintainerr": {
			{name: "maintainerr_collections", interval: 10 * time.Minute, run: (*Poller).runMaintainerrCollections},
		},
		"tailscale": {
			{name: "tailscale_devices", interval: 60 * time.Second, run: (*Poller).runTailscaleDevices},
		},
	}

	return p
}

func (p *Poller) Start(ctx context.Context) {
	go p.run(ctx)
}

func (p *Poller) Refresh(instanceID string, kind RefreshKind) {
	select {
	case p.refreshCh <- refreshReq{instanceID: instanceID, kind: kind}:
	default:
	}
}

func (p *Poller) run(ctx context.Context) {
	log.Info().Msg("poller started")
	defer log.Info().Msg("poller stopped")

	sem := make(chan struct{}, pollerMaxConcurrentUpst) // cap concurrent upstream calls

	// small tick; jobs self-throttle by lastRun
	t := time.NewTicker(pollerTickInterval)
	defer t.Stop()

	// initial blast
	p.tick(ctx, sem, true, "", RefreshAll)

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-p.refreshCh:
			p.tick(ctx, sem, true, req.instanceID, req.kind)
		case <-t.C:
			p.tick(ctx, sem, false, "", RefreshAll)
		}
	}
}

func (p *Poller) tick(ctx context.Context, sem chan struct{}, force bool, onlyInstance string, kind RefreshKind) {
	services := p.getServices(ctx, force || onlyInstance != "")
	if services == nil {
		return
	}

	for _, svc := range services {
		if onlyInstance != "" && svc.InstanceID != onlyInstance {
			continue
		}

		serviceType, _, _ := strings.Cut(svc.InstanceID, "-")
		configured := isServiceConfigured(serviceType, svc)

		if kind == RefreshAll || kind == RefreshHealth {
			if configured {
				p.maybeRun(ctx, sem, svc, serviceType, "health", 30*time.Second, force, (*Poller).runHealth)
			} else {
				p.maybeRun(ctx, sem, svc, serviceType, "health", 60*time.Second, force, (*Poller).runPending)
			}
		}

		if !configured {
			continue
		}

		if !(kind == RefreshAll || kind == RefreshStats) {
			continue
		}

		for _, job := range p.jobs[serviceType] {
			p.maybeRun(ctx, sem, svc, serviceType, job.name, job.interval, force, job.run)
		}
	}
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

func (p *Poller) maybeRun(ctx context.Context, sem chan struct{}, svc models.ServiceConfiguration, serviceType string, job string, interval time.Duration, force bool, run jobRunner) {
	key := svc.InstanceID + ":" + job

	p.mu.Lock()
	if p.inFlight[key] {
		p.mu.Unlock()
		return
	}
	last := p.lastRun[key]
	due := force || last.IsZero() || time.Since(last) >= interval
	if !due {
		p.mu.Unlock()
		return
	}
	p.inFlight[key] = true
	p.lastRun[key] = time.Now()
	p.mu.Unlock()

	go func() {
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

		jobCtx, cancel := context.WithTimeout(ctx, pollerJobTimeout)
		defer cancel()

		run(p, jobCtx, svc, serviceType)
	}()
}

func (p *Poller) runHealth(ctx context.Context, svc models.ServiceConfiguration, serviceType string) {
	checker := p.registry.CreateService(serviceType)
	if checker == nil {
		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "error",
			Message:     "Unsupported service type: " + serviceType,
			LastChecked: time.Now(),
		})
		return
	}

	health, _ := checker.CheckHealth(ctx, svc.URL, svc.APIKey)
	health.ServiceID = svc.InstanceID
	if health.LastChecked.IsZero() {
		health.LastChecked = time.Now()
	}
	p.bc.Publish(health)
}

func (p *Poller) runPending(_ context.Context, svc models.ServiceConfiguration, _ string) {
	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "pending",
		Message:     "Service not configured",
		LastChecked: time.Now(),
	})
}

func (p *Poller) runPlexSessions(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &plex.PlexService{}
	sessions, err := service.GetSessions(ctx, svc.URL, svc.APIKey)
	if err != nil || sessions == nil {
		return
	}

	metadata := sessions.MediaContainer.Metadata
	if metadata == nil {
		metadata = []types.PlexSession{}
	}

	transcoding := 0
	for _, s := range metadata {
		if s.TranscodeSession != nil {
			transcoding++
		}
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "plex_sessions",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"plex": map[string]interface{}{
				"sessions": metadata,
			},
		},
		Details: map[string]interface{}{
			"plex": map[string]interface{}{
				"activeStreams": len(metadata),
				"transcoding":   transcoding,
			},
		},
	})
}

func (p *Poller) runOverseerrRequests(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &overseerr.OverseerrService{}
	service.SetDB(p.db)

	stats, err := service.GetRequests(ctx, svc.URL, svc.APIKey)
	if err != nil || stats == nil {
		return
	}

	if stats.Requests == nil {
		stats.Requests = []types.MediaRequest{}
	}

	status := "online"
	if stats.PendingCount > 0 {
		status = "warning"
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      status,
		Message:     "overseerr_requests",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"overseerr": types.OverseerrStats{
				Requests:     stats.Requests,
				PendingCount: stats.PendingCount,
			},
		},
		Details: map[string]interface{}{
			"overseerr": types.OverseerrDetails{
				PendingCount:  stats.PendingCount,
				TotalRequests: len(stats.Requests),
			},
		},
	})
}

func (p *Poller) runRadarrQueue(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &radarr.RadarrService{}
	records, err := service.GetQueueForHealth(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return
	}
	if records == nil {
		records = []types.RadarrQueueRecord{}
	}

	resp := &types.RadarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	var totalSize int64
	downloading := 0
	for _, r := range records {
		totalSize += r.Size
		if r.Status == "downloading" {
			downloading++
		}
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "radarr_queue",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"radarr": map[string]interface{}{
				"queue": resp,
			},
		},
		Details: map[string]interface{}{
			"radarr": map[string]interface{}{
				"queueCount":       resp.TotalRecords,
				"totalRecords":     resp.TotalRecords,
				"downloadingCount": downloading,
				"totalSize":        totalSize,
			},
		},
	})
}

func (p *Poller) runSonarrQueue(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &sonarr.SonarrService{}
	records, err := service.GetQueueForHealth(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return
	}
	if records == nil {
		records = []types.QueueRecord{}
	}

	resp := &types.SonarrQueueResponse{
		Records:      records,
		TotalRecords: len(records),
	}

	var totalSize int64
	downloading := 0
	episodeCount := 0
	for _, r := range records {
		totalSize += r.Size
		if r.Status == "downloading" {
			downloading++
		}
		episodeCount += len(r.Episodes)
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "sonarr_queue",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queue": resp,
			},
		},
		Details: map[string]interface{}{
			"sonarr": map[string]interface{}{
				"queueCount":       resp.TotalRecords,
				"totalRecords":     resp.TotalRecords,
				"downloadingCount": downloading,
				"episodeCount":     episodeCount,
				"totalSize":        totalSize,
			},
		},
	})
}

func (p *Poller) runProwlarr(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	now := time.Now()

	// Indexers list
	indexerURL := fmt.Sprintf("%s/api/v1/indexer", strings.TrimRight(svc.URL, "/"))
	resp, err := arr.MakeArrRequest(ctx, http.MethodGet, indexerURL, svc.APIKey, nil)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	var indexers []types.ProwlarrIndexer
	if err := json.NewDecoder(resp.Body).Decode(&indexers); err != nil {
		return
	}
	if indexers == nil {
		indexers = []types.ProwlarrIndexer{}
	}

	// Indexer stats (and derived totals)
	ps := prowlarr.NewProwlarrService().(*prowlarr.ProwlarrService)
	idxStats, err := ps.GetIndexerStats(ctx, svc.URL, svc.APIKey)
	if err == nil && idxStats != nil {
		statsMap := make(map[int]types.ProwlarrIndexerStats, len(idxStats.Indexers))
		totalGrabs := 0
		totalFails := 0
		for _, stat := range idxStats.Indexers {
			statsMap[stat.IndexerID] = stat
			totalGrabs += stat.NumberOfGrabs
			totalFails += stat.NumberOfFailedGrabs
		}
		for i := range indexers {
			if st, ok := statsMap[indexers[i].ID]; ok {
				indexers[i].AverageResponseTime = st.AverageResponseTime
				indexers[i].NumberOfGrabs = st.NumberOfGrabs
				indexers[i].NumberOfQueries = st.NumberOfQueries
			}
		}

		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "online",
			Message:     "prowlarr_stats",
			LastChecked: now,
			Stats: map[string]interface{}{
				"prowlarr": map[string]interface{}{
					"stats": types.ProwlarrStatsResponse{
						GrabCount:    totalGrabs,
						FailCount:    totalFails,
						IndexerCount: len(indexers),
					},
				},
			},
		})
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "prowlarr_indexers",
		LastChecked: now,
		Stats: map[string]interface{}{
			"prowlarr": map[string]interface{}{
				"indexers": indexers,
			},
		},
	})
}

func (p *Poller) runAutobrr(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &autobrr.AutobrrService{}

	// Stats
	if stats, err := service.GetReleaseStats(ctx, svc.URL, svc.APIKey); err == nil {
		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "online",
			Message:     "autobrr_stats",
			LastChecked: time.Now(),
			Stats: map[string]interface{}{
				"autobrr": stats,
			},
		})
	}

	// IRC
	if irc, err := service.GetIRCStatus(ctx, svc.URL, svc.APIKey); err == nil {
		status := "online"
		for _, s := range irc {
			if !s.Healthy && s.Enabled {
				status = "warning"
				break
			}
		}
		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      status,
			Message:     "autobrr_irc_status",
			LastChecked: time.Now(),
			Details: map[string]interface{}{
				"autobrr": types.AutobrrDetails{IRC: irc},
			},
		})
	}

	// Releases
	if releases, err := service.GetReleases(ctx, svc.URL, svc.APIKey); err == nil {
		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "online",
			Message:     "autobrr_releases",
			LastChecked: time.Now(),
			Stats: map[string]interface{}{
				"autobrr": releases,
			},
		})
	}
}

func (p *Poller) runMaintainerrCollections(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &maintainerr.MaintainerrService{}
	collections, err := service.GetCollections(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return
	}
	if collections == nil {
		collections = []maintainerr.Collection{}
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "maintainerr_collections",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collections": collections,
			},
		},
		Details: map[string]interface{}{
			"maintainerr": map[string]interface{}{
				"collectionCount": len(collections),
			},
		},
	})
}

func (p *Poller) runTailscaleDevices(ctx context.Context, svc models.ServiceConfiguration, _ string) {
	service := &tailscale.TailscaleService{}
	devices, err := service.GetDevices(ctx, svc.URL, svc.APIKey)
	if err != nil {
		return
	}
	if devices == nil {
		devices = []tailscale.Device{}
	}

	online := 0
	for _, d := range devices {
		if d.Online {
			online++
		}
	}

	p.bc.Publish(models.ServiceHealth{
		ServiceID:   svc.InstanceID,
		Status:      "online",
		Message:     "tailscale_devices",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"tailscale": map[string]interface{}{
				"devices": devices,
			},
		},
		Details: map[string]interface{}{
			"tailscale": map[string]interface{}{
				"total":  len(devices),
				"online": online,
			},
		},
	})
}
