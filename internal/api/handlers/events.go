// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services"
)

type EventsHandler struct {
	db     *database.DB
	health *services.HealthService
}

func NewEventsHandler(db *database.DB, health *services.HealthService) *EventsHandler {
	handler := &EventsHandler{
		db:     db,
		health: health,
	}
	return handler
}

type client struct {
	send        chan models.ServiceHealth
	done        chan struct{}
	connectedAt time.Time
	lastActive  time.Time // Track last successful message send
}

var (
	clients   = make(map[*client]bool)
	clientsMu sync.RWMutex

	// Track active client count
	activeClients atomic.Int64

	// Reduced concurrent checks from 10 to 5 to prevent overwhelming
	healthCheckSemaphore = make(chan struct{}, 5)

	// Track last check time per service
	lastChecks   = make(map[string]time.Time)
	lastChecksMu sync.RWMutex

	// Client cleanup ticker
	cleanupTicker *time.Ticker
)

const (
	minCheckInterval  = 30 * time.Second
	checkTimeout      = 10 * time.Second // Reduced from 15s to 10s
	keepAliveInterval = 15 * time.Second
	broadcastTimeout  = 2 * time.Second  // Reduced from 5s to 2s
	clientBufferSize  = 50               // Reduced from 100 to 50
	cleanupInterval   = 2 * time.Minute  // More frequent cleanup
	maxClientAge      = 10 * time.Minute // Max time before forcing reconnect
	maxInactiveTime   = 30 * time.Second // Max time without successful message
)

// safeClose safely closes a channel if it's not already closed
func safeClose(ch chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Interface("recover", r).Msg("Recovered from panic while closing channel")
		}
	}()

	select {
	case <-ch: // channel already closed
		return
	default:
		close(ch)
	}
}

// startClientCleanup starts periodic cleanup of disconnected clients
func startClientCleanup() {
	if cleanupTicker != nil {
		return
	}

	cleanupTicker = time.NewTicker(cleanupInterval)
	go func() {
		for range cleanupTicker.C {
			cleanupClients()
		}
	}()
}

// cleanupClients removes disconnected and stale clients
func cleanupClients() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	before := len(clients)
	now := time.Now()

	for client := range clients {
		select {
		case <-client.done:
			delete(clients, client)
			activeClients.Add(-1)
		default:
			// Force reconnect for old connections
			if now.Sub(client.connectedAt) > maxClientAge {
				log.Info().
					Time("connected_at", client.connectedAt).
					Msg("Forcing reconnect for old SSE connection")
				safeClose(client.done)
				delete(clients, client)
				activeClients.Add(-1)
				continue
			}

			// Remove inactive clients
			if now.Sub(client.lastActive) > maxInactiveTime {
				log.Info().
					Time("connected_at", client.connectedAt).
					Time("last_active", client.lastActive).
					Msg("Removing inactive SSE client")
				safeClose(client.done)
				delete(clients, client)
				activeClients.Add(-1)
			}
		}
	}

	after := len(clients)
	if before != after {
		log.Info().
			Int("before", before).
			Int("after", after).
			Int("cleaned", before-after).
			Msg("Cleaned up SSE clients")
	}
}

// processServiceBatch handles health checks for a batch of services
func (h *EventsHandler) processServiceBatch(ctx context.Context, services []models.ServiceConfiguration, results chan<- models.ServiceHealth, wg *sync.WaitGroup) {
	for _, service := range services {
		if service.URL == "" {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
			wg.Add(1)
			go h.checkSingleService(ctx, service, results, wg)
		}
	}
}

// checkSingleService performs health check for a single service
func (h *EventsHandler) checkSingleService(ctx context.Context, svc models.ServiceConfiguration, results chan<- models.ServiceHealth, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create timeout context for health check
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	select {
	case healthCheckSemaphore <- struct{}{}:
		defer func() { <-healthCheckSemaphore }()

		serviceType := strings.Split(svc.InstanceID, "-")[0]
		serviceHealth := models.ServiceHealth{
			ServiceID:   svc.InstanceID,
			Status:      "checking",
			LastChecked: time.Now(),
		}

		if serviceChecker := models.NewServiceRegistry().CreateService(serviceType); serviceChecker != nil {
			health, statusCode := serviceChecker.CheckHealth(checkCtx, svc.URL, svc.APIKey)
			health.ServiceID = svc.InstanceID

			if statusCode != 200 {
				log.Debug().
					Int("status_code", statusCode).
					Str("service", svc.InstanceID).
					Msg("Health check failed")
				health.Status = "error"
				health.Message = "Service returned non-200 status code"
			}

			lastChecksMu.Lock()
			lastChecks[svc.InstanceID] = time.Now()
			lastChecksMu.Unlock()

			select {
			case results <- health:
			case <-checkCtx.Done():
				return
			}
		} else {
			serviceHealth.Status = "error"
			serviceHealth.Message = "Unsupported service type: " + serviceType
			select {
			case results <- serviceHealth:
			case <-checkCtx.Done():
			}
		}
	case <-time.After(5 * time.Second): // Reduced timeout
		log.Debug().Str("service", svc.InstanceID).Msg("Health check skipped due to concurrency limit")
	case <-checkCtx.Done():
		log.Debug().Str("service", svc.InstanceID).Msg("Health check cancelled")
	}
}

// collectResults gathers health check results with timeout
func (h *EventsHandler) collectResults(ctx context.Context, results <-chan models.ServiceHealth) []models.ServiceHealth {
	var allResults []models.ServiceHealth
	resultsTimer := time.NewTimer(3 * time.Second) // Reduced collection timeout
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

// checkAndBroadcastHealth performs health checks for all services and broadcasts results
func (h *EventsHandler) checkAndBroadcastHealth(ctx context.Context) []models.ServiceHealth {
	services, err := h.db.GetAllServices(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching services")
		return nil
	}

	if len(services) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	results := make(chan models.ServiceHealth, len(services))
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Overall timeout for batch
	defer cancel()

	// Process services in smaller batches
	batchSize := 3 // Reduced batch size
	for i := 0; i < len(services); i += batchSize {
		end := i + batchSize
		if end > len(services) {
			end = len(services)
		}

		h.processServiceBatch(checkCtx, services[i:end], results, &wg)

		// Wait for batch completion or context cancellation
		if !h.waitForBatch(checkCtx, &wg) {
			return nil
		}

		// Increased delay between batches
		time.Sleep(time.Second)
	}

	// Close results channel after all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	return h.collectResults(checkCtx, results)
}

// waitForBatch waits for the current batch to complete or context to be canceled
func (h *EventsHandler) waitForBatch(ctx context.Context, wg *sync.WaitGroup) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	case <-time.After(5 * time.Second): // Added timeout for batch wait
		log.Warn().Msg("Batch wait timeout")
		return false
	}
}

// StreamHealth handles SSE connections for real-time health updates
func (h *EventsHandler) StreamHealth(c *gin.Context) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no") // Disable proxy buffering

	// Create new client with buffered channel and done signal
	client := &client{
		send:        make(chan models.ServiceHealth, clientBufferSize),
		done:        make(chan struct{}),
		connectedAt: time.Now(),
		lastActive:  time.Now(),
	}

	clientsMu.Lock()
	clients[client] = true
	currentClients := activeClients.Add(1)
	clientsMu.Unlock()

	// Log new connection
	log.Info().
		Time("connected_at", client.connectedAt).
		Int64("total_clients", currentClients).
		Msg("New SSE client connected")

	ctx := c.Request.Context()

	// Ensure cleanup on connection close
	go func() {
		<-ctx.Done()
		clientsMu.Lock()
		delete(clients, client)
		currentClients := activeClients.Add(-1)
		clientsMu.Unlock()
		safeClose(client.done)
		close(client.send)

		log.Info().
			Time("connected_at", client.connectedAt).
			Time("disconnected_at", time.Now()).
			Int64("total_clients", currentClients).
			Msg("SSE client disconnected")
	}()

	// Perform immediate health check for new connection
	go h.checkAndBroadcastHealth(ctx)

	lastUpdate := make(map[string]time.Time)
	keepAliveTicker := time.NewTicker(keepAliveInterval)
	defer keepAliveTicker.Stop()

	healthCheckTicker := time.NewTicker(minCheckInterval)
	defer healthCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case msg, ok := <-client.send:
			if !ok {
				return
			}

			now := time.Now()
			if lastUpdateTime, exists := lastUpdate[msg.ServiceID]; !exists || now.Sub(lastUpdateTime) >= 5*time.Second {
				data, err := json.Marshal(msg)
				if err != nil {
					log.Error().Err(err).Msg("Failed to marshal health message")
					continue
				}
				lastUpdate[msg.ServiceID] = now

				// Update last active time on successful send
				client.lastActive = now

				c.SSEvent("health", string(data))
				c.Writer.Flush()
			}
		case <-keepAliveTicker.C:
			select {
			case <-ctx.Done():
				return
			default:
				c.SSEvent("keepalive", time.Now().Unix())
				c.Writer.Flush()
			}
		case <-healthCheckTicker.C:
			select {
			case <-ctx.Done():
				return
			default:
				go h.checkAndBroadcastHealth(ctx)
			}
		}
	}
}

// BroadcastHealth sends health updates to all connected clients
func BroadcastHealth(health models.ServiceHealth) {
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

var (
	healthMonitor     *time.Ticker
	healthMonitorOnce sync.Once
	monitorCtx        context.Context
	monitorCancel     context.CancelFunc
)

// StartHealthMonitor starts the background health check process
func (h *EventsHandler) StartHealthMonitor() {
	healthMonitorOnce.Do(func() {
		monitorCtx, monitorCancel = context.WithCancel(context.Background())

		// Start client cleanup
		startClientCleanup()

		go h.checkAndBroadcastHealth(monitorCtx)

		healthMonitor = time.NewTicker(minCheckInterval)
		go func() {
			for {
				select {
				case <-healthMonitor.C:
					h.checkAndBroadcastHealth(monitorCtx)
				case <-monitorCtx.Done():
					return
				}
			}
		}()

		log.Info().Msg("Health monitor started with client cleanup")
	})
}

// StopHealthMonitor stops the health monitoring
func (h *EventsHandler) StopHealthMonitor() {
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
