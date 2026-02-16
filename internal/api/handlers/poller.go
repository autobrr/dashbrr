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
	"github.com/autobrr/dashbrr/internal/services/cache"
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

type Poller struct {
	db    *database.DB
	cache cache.Store
	bc    *Broadcaster

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

func NewPoller(db *database.DB, cache cache.Store, bc *Broadcaster) *Poller {
	return &Poller{
		db:        db,
		cache:     cache,
		bc:        bc,
		lastRun:   make(map[string]time.Time),
		inFlight:  make(map[string]bool),
		refreshCh: make(chan refreshReq, 64),
	}
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

	sem := make(chan struct{}, 4) // cap concurrent upstream calls

	// small tick; jobs self-throttle by lastRun
	t := time.NewTicker(1 * time.Second)
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

		serviceType := strings.Split(svc.InstanceID, "-")[0]

		if kind == RefreshAll || kind == RefreshHealth {
			if svc.URL == "" || (svc.APIKey == "" && serviceType != "general") {
				p.maybeRun(ctx, sem, svc, "health", 60*time.Second, force, p.jobPending(svc))
			} else {
				p.maybeRun(ctx, sem, svc, "health", 30*time.Second, force, p.jobHealth(serviceType, svc))
			}
		}

		if svc.URL == "" || (svc.APIKey == "" && serviceType != "general") {
			continue
		}

		if !(kind == RefreshAll || kind == RefreshStats) {
			continue
		}

		switch serviceType {
		case "plex":
			p.maybeRun(ctx, sem, svc, "plex_sessions", 10*time.Second, force, p.jobPlexSessions(svc))
		case "overseerr":
			p.maybeRun(ctx, sem, svc, "overseerr_requests", 60*time.Second, force, p.jobOverseerrRequests(svc))
		case "radarr":
			p.maybeRun(ctx, sem, svc, "radarr_queue", 60*time.Second, force, p.jobRadarrQueue(svc))
		case "sonarr":
			p.maybeRun(ctx, sem, svc, "sonarr_queue", 60*time.Second, force, p.jobSonarrQueue(svc))
		case "prowlarr":
			p.maybeRun(ctx, sem, svc, "prowlarr", 120*time.Second, force, p.jobProwlarr(svc))
		case "autobrr":
			p.maybeRun(ctx, sem, svc, "autobrr", 120*time.Second, force, p.jobAutobrr(svc))
		case "maintainerr":
			p.maybeRun(ctx, sem, svc, "maintainerr_collections", 10*time.Minute, force, p.jobMaintainerrCollections(svc))
		case "tailscale":
			p.maybeRun(ctx, sem, svc, "tailscale_devices", 60*time.Second, force, p.jobTailscaleDevices(svc))
		}
	}
}

func (p *Poller) getServices(ctx context.Context, force bool) []models.ServiceConfiguration {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reload config periodically or when forced.
	if !force && time.Since(p.loadedAt) < 15*time.Second && p.services != nil {
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

func (p *Poller) maybeRun(ctx context.Context, sem chan struct{}, svc models.ServiceConfiguration, job string, interval time.Duration, force bool, fn func(context.Context)) {
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
		sem <- struct{}{}
		defer func() { <-sem }()
		defer func() {
			p.mu.Lock()
			delete(p.inFlight, key)
			p.mu.Unlock()
		}()

		jobCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		fn(jobCtx)
	}()
}

func (p *Poller) jobHealth(serviceType string, svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
		checker := models.NewServiceRegistry().CreateService(serviceType)
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
}

func (p *Poller) jobPending(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
		_ = ctx
		p.bc.Publish(models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "pending",
			Message:     "Service not configured",
			LastChecked: time.Now(),
		})
	}
}

func (p *Poller) jobPlexSessions(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobOverseerrRequests(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobRadarrQueue(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobSonarrQueue(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobProwlarr(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
		// Indexers list
		indexerURL := fmt.Sprintf("%s/api/v1/indexer", strings.TrimRight(svc.URL, "/"))
		resp, err := arr.MakeArrRequest(ctx, http.MethodGet, indexerURL, svc.APIKey, nil)
		if err != nil {
			return
		}
		defer resp.Body.Close()

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
			statsMap := make(map[int]types.ProwlarrIndexerStats)
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
				ServiceID: svc.InstanceID,
				Status:    "online",
				Message:   "prowlarr_stats",
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
			ServiceID: svc.InstanceID,
			Status:    "online",
			Message:   "prowlarr_indexers",
			Stats: map[string]interface{}{
				"prowlarr": map[string]interface{}{
					"indexers": indexers,
				},
			},
		})
	}
}

func (p *Poller) jobAutobrr(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobMaintainerrCollections(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}

func (p *Poller) jobTailscaleDevices(svc models.ServiceConfiguration) func(context.Context) {
	return func(ctx context.Context) {
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
}
