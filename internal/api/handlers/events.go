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
	send chan models.ServiceHealth
	done chan struct{}
	// Add connection time for monitoring
	connectedAt time.Time
}

var (
	clients   = make(map[*client]bool)
	clientsMu sync.RWMutex

	// Track active client count
	activeClients int64

	// Increased concurrent checks from 5 to 10
	healthCheckSemaphore = make(chan struct{}, 10)

	// Track last check time per service
	lastChecks   = make(map[string]time.Time)
	lastChecksMu sync.RWMutex

	// Client cleanup ticker
	cleanupTicker *time.Ticker
)

const (
	minCheckInterval  = 30 * time.Second
	checkTimeout      = 15 * time.Second
	keepAliveInterval = 15 * time.Second
	broadcastTimeout  = 5 * time.Second // Increased from 2s to 5s
	clientBufferSize  = 100             // Increased from 20 to 100
	cleanupInterval   = 5 * time.Minute
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

// cleanupClients removes disconnected clients and logs metrics
func cleanupClients() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	before := len(clients)
	now := time.Now()

	for client := range clients {
		select {
		case <-client.done:
			delete(clients, client)
			atomic.AddInt64(&activeClients, -1)
		default:
			// Log clients that haven't received messages for too long
			if now.Sub(client.connectedAt) > 30*time.Minute {
				log.Warn().
					Time("connected_at", client.connectedAt).
					Msg("Long-running SSE connection detected")
			}
		}
	}

	after := len(clients)
	if before != after {
		log.Info().
			Int("before", before).
			Int("after", after).
			Int("cleaned", before-after).
			Msg("Cleaned up disconnected SSE clients")
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
	// Make results channel buffered to prevent blocking
	results := make(chan models.ServiceHealth, len(services))
	allResults := make([]models.ServiceHealth, 0, len(services))

	// Create a context that will be cancelled when the parent context is done
	checkCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Increased batch size from 3 to 5
	batchSize := 5
	for i := 0; i < len(services); i += batchSize {
		end := i + batchSize
		if end > len(services) {
			end = len(services)
		}

		batch := services[i:end]
		for _, service := range batch {
			if service.URL == "" {
				continue
			}

			wg.Add(1)
			go func(svc models.ServiceConfiguration) {
				defer wg.Done()

				select {
				case healthCheckSemaphore <- struct{}{}:
					defer func() { <-healthCheckSemaphore }()

					serviceType := strings.Split(svc.InstanceID, "-")[0]
					serviceHealth := models.ServiceHealth{
						ServiceID:   svc.InstanceID,
						Status:      "checking",
						LastChecked: time.Now(),
					}

					serviceChecker := models.NewServiceRegistry().CreateService(serviceType)
					if serviceChecker != nil {
						health, _ := serviceChecker.CheckHealth(checkCtx, svc.URL, svc.APIKey)
						health.ServiceID = svc.InstanceID

						lastChecksMu.Lock()
						lastChecks[svc.InstanceID] = time.Now()
						lastChecksMu.Unlock()

						select {
						case results <- health:
						case <-checkCtx.Done():
							log.Warn().
								Str("service", svc.InstanceID).
								Msg("Context cancelled while sending results")
							return
						}
					} else {
						serviceHealth.Status = "error"
						serviceHealth.Message = "Unsupported service type: " + serviceType
						select {
						case results <- serviceHealth:
						case <-checkCtx.Done():
							return
						}
					}
				case <-time.After(10 * time.Second):
					log.Warn().
						Str("service", svc.InstanceID).
						Msg("Health check skipped due to concurrency limit")
				case <-checkCtx.Done():
					return
				}
			}(service)
		}

		// Wait for current batch to complete or context to be cancelled
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Batch completed successfully
		case <-checkCtx.Done():
			return allResults
		}

		// Reduced delay between batches
		time.Sleep(500 * time.Millisecond)
	}

	// Close results channel after all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results with timeout
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
		case <-checkCtx.Done():
			return allResults
		}
	}
}

// StreamHealth handles SSE connections for real-time health updates
func (h *EventsHandler) StreamHealth(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create new client with buffered channel and done signal
	client := &client{
		send:        make(chan models.ServiceHealth, clientBufferSize),
		done:        make(chan struct{}),
		connectedAt: time.Now(),
	}

	clientsMu.Lock()
	clients[client] = true
	atomic.AddInt64(&activeClients, 1)
	currentClients := len(clients)
	clientsMu.Unlock()

	// Log new connection
	log.Info().
		Time("connected_at", client.connectedAt).
		Int("total_clients", currentClients).
		Msg("New SSE client connected")

	ctx := c.Request.Context()

	// Ensure cleanup on connection close
	go func() {
		<-ctx.Done()
		clientsMu.Lock()
		delete(clients, client)
		currentClients := len(clients)
		clientsMu.Unlock()
		atomic.AddInt64(&activeClients, -1)
		safeClose(client.done)
		close(client.send)

		// Log disconnection
		log.Info().
			Time("connected_at", client.connectedAt).
			Time("disconnected_at", time.Now()).
			Int("total_clients", currentClients).
			Msg("SSE client disconnected")
	}()

	// Perform immediate health check for new connection
	go h.checkAndBroadcastHealth(ctx)

	lastUpdate := make(map[string]time.Time)
	keepAliveTicker := time.NewTicker(keepAliveInterval)
	defer keepAliveTicker.Stop()

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

		healthMonitor = time.NewTicker(30 * time.Second)
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
