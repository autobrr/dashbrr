// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/internal/utils"

	"github.com/rs/zerolog/log"
)

// ServiceHealthChecker defines the interface for service health checking
type ServiceHealthChecker interface {
	CheckHealth(ctx context.Context, url, apiKey string) (types.ServiceHealth, int)
}

const (
	minCheckInterval  = 60 * time.Second // Increased to reduce connection frequency
	checkTimeout      = 10 * time.Second
	keepAliveInterval = 15 * time.Second
	broadcastTimeout  = 2 * time.Second
	clientBufferSize  = 10 // Reduced buffer size
	cleanupInterval   = 2 * time.Minute
	maxClientAge      = 10 * time.Minute
	maxInactiveTime   = 30 * time.Second
)

func (m *ServiceManager) StartHealthMonitor() {
	monitorCtx, monitorCancel = context.WithCancel(context.Background())

	// Start client cleanup
	//startClientCleanup()

	// TODO remove?
	go m.checkAndBroadcastHealth(monitorCtx)

	healthMonitor := time.NewTicker(minCheckInterval)
	go func() {
		for {
			select {
			case <-healthMonitor.C:
				log.Trace().Msg("Health monitor tick, check and broadcast health")
				m.checkAndBroadcastHealth(monitorCtx)
			case <-monitorCtx.Done():
				return
			}
		}
	}()

	log.Info().Msg("Health monitor started with client cleanup")
}

// StopHealthMonitor stops the health monitoring
func (m *ServiceManager) StopHealthMonitor() {
	if healthMonitor != nil {
		healthMonitor.Stop()
	}
	if cleanupTicker != nil {
		cleanupTicker.Stop()
		cleanupTicker = nil
	}
	if monitorCancel != nil {
		monitorCancel()
	}
	log.Info().Msg("Health monitor and client cleanup stopped")
}

// checkAndBroadcastHealth performs health checks for all services and broadcasts results
func (m *ServiceManager) checkAndBroadcastHealth(ctx context.Context) []types.ServiceHealth {
	log.Trace().Msg("check and broadcast health")

	allServices, err := m.db.GetAllServices(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching allServices")
		return nil
	}

	if len(allServices) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	results := make(chan types.ServiceHealth, len(allServices))
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Increased timeout for sequential processing
	defer cancel()

	// Process all allServices in a single batch
	m.processServiceBatch(checkCtx, allServices, results, &wg)

	// Close results channel after all allServices are processed
	go func() {
		wg.Wait()
		close(results)
	}()

	return m.collectResults(checkCtx, results)
}

// extractServiceType safely extracts the service type from an instance ID
func extractServiceType(instanceID string) (string, error) {
	parts := strings.Split(instanceID, "-")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid instance ID format: %s", instanceID)
	}
	return parts[0], nil
}

// processServiceBatch handles health checks for a batch of services
func (m *ServiceManager) processServiceBatch(ctx context.Context, services []types.ServiceConfiguration, results chan<- types.ServiceHealth, wg *sync.WaitGroup) {
	// Process services sequentially within batch to prevent connection spikes
	for _, service := range services {
		if service.URL == "" {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
			wg.Add(1)
			// Run synchronously to prevent connection spikes
			m.checkSingleService(ctx, service, results, wg)
		}
	}
}

type client struct {
	send        chan types.ServiceHealth
	done        chan struct{}
	connectedAt time.Time
	lastActive  time.Time // Track last successful message send
}

var (
	clients   = make(map[*client]bool)
	clientsMu sync.RWMutex

	// Track active client count
	activeClients atomic.Int64

	// Reduced concurrent checks to prevent connection leaks
	healthCheckSemaphore = make(chan struct{}, 2)

	// Track last check time per service
	lastChecks   = make(map[string]time.Time)
	lastChecksMu sync.RWMutex

	healthMonitor     *time.Ticker
	healthMonitorOnce sync.Once
	monitorCtx        context.Context
	monitorCancel     context.CancelFunc

	// Client cleanup ticker
	cleanupTicker *time.Ticker
)

// checkSingleService performs health check for a single service
func (m *ServiceManager) checkSingleService(ctx context.Context, svc types.ServiceConfiguration, results chan<- types.ServiceHealth, wg *sync.WaitGroup) {
	log.Trace().Str("service", svc.InstanceID).Msg("EventsHandler: Checking single service")
	defer wg.Done()

	// Skip if checked recently
	lastChecksMu.RLock()
	if lastCheck, exists := lastChecks[svc.InstanceID]; exists {
		if time.Since(lastCheck) < minCheckInterval {
			lastChecksMu.RUnlock()
			return
		}
	}
	lastChecksMu.RUnlock()

	// Create timeout context for health check
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	select {
	case healthCheckSemaphore <- struct{}{}:
		defer func() { <-healthCheckSemaphore }()

		serviceType, err := extractServiceType(svc.InstanceID)
		if err != nil {
			log.Error().Err(err).Str("instance_id", svc.InstanceID).Msg("Failed to extract service type")
			results <- types.ServiceHealth{
				ServiceID:   svc.InstanceID,
				Status:      "error",
				Message:     "Invalid service ID format",
				LastChecked: time.Now(),
			}
			return
		}

		serviceHealth := types.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "checking",
			LastChecked: time.Now(),
		}

		log.Trace().Str("service", svc.InstanceID).Str("type", serviceType).Msg("EventsHandler: checkSingleService")

		// TODO move all this to service manager itself?
		serviceChecker, err := m.GetServiceHealthChecker(svc.InstanceID)
		if err != nil {
			log.Error().Err(err).Str("service", svc.InstanceID).Str("type", serviceType).Msg("Failed to get service checker")
			serviceHealth.Status = "error"
			serviceHealth.Message = "Unsupported service type: " + serviceType
			select {
			case results <- serviceHealth:
			case <-checkCtx.Done():
			}

			return
		}

		health, statusCode := serviceChecker.CheckHealth(checkCtx, svc.URL, svc.APIKey)

		// Safely convert health to ServiceHealth
		convertedHealth, err := utils.SafeStructConvert[types.ServiceHealth](health)
		if err != nil {
			log.Error().
				Err(err).
				Str("service", svc.InstanceID).
				Str("type", utils.GetTypeString(health)).
				Msg("Failed to convert health check result")

			serviceHealth.Status = "error"
			serviceHealth.Message = "Failed to process health check result"
			select {
			case results <- serviceHealth:
			case <-checkCtx.Done():
			}
			return
		}

		convertedHealth.ServiceID = svc.InstanceID

		if statusCode != 200 {
			log.Debug().
				Int("status_code", statusCode).
				Str("service", svc.InstanceID).
				Msg("Health check failed")
			convertedHealth.Status = "error"
			convertedHealth.Message = "Service returned non-200 status code"
		}

		lastChecksMu.Lock()
		lastChecks[svc.InstanceID] = time.Now()
		lastChecksMu.Unlock()

		select {
		case results <- convertedHealth:
		case <-checkCtx.Done():
			return
		}
	case <-checkCtx.Done():
		log.Debug().Str("service", svc.InstanceID).Msg("Health check cancelled")
	}
}

// collectResults gathers health check results with timeout
func (m *ServiceManager) collectResults(ctx context.Context, results <-chan types.ServiceHealth) []types.ServiceHealth {
	var allResults []types.ServiceHealth
	resultsTimer := time.NewTimer(5 * time.Second)
	defer resultsTimer.Stop()

	for {
		select {
		case health, ok := <-results:
			if !ok {
				return allResults
			}
			if health.ResponseTime > 0 || health.Status != "" {
				allResults = append(allResults, health)
				BroadcastHealth(health)
			}
		case <-resultsTimer.C:
			return allResults
		case <-ctx.Done():
			return allResults
		}
	}
}

// BroadcastHealth sends health updates to all connected clients
func BroadcastHealth(health types.ServiceHealth) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	for client := range clients {
		select {
		case <-client.done:
			continue
		case client.send <- health:
			// Message sent successfully
		case <-time.After(broadcastTimeout):
			log.Debug().
				Str("service", health.ServiceID).
				Time("client_connected_at", client.connectedAt).
				Msg("Skipped broadcast due to slow client")
		}
	}
}
