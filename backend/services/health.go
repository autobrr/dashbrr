// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/models"
)

var serviceFactory = models.NewServiceFactory()

type HealthService struct {
	mu                sync.RWMutex
	monitoredServices map[string]context.CancelFunc
	healthChecks      map[string]*HealthCheck
}

type HealthCheck struct {
	Status          string    `json:"status"`
	ResponseTime    int64     `json:"responseTime"`
	Message         string    `json:"message,omitempty"`
	Version         string    `json:"version,omitempty"`
	LastChecked     time.Time `json:"lastChecked"`
	UpdateAvailable bool      `json:"updateAvailable,omitempty"`
}

func NewHealthService() *HealthService {
	return &HealthService{
		monitoredServices: make(map[string]context.CancelFunc),
		healthChecks:      make(map[string]*HealthCheck),
	}
}

// CheckServiceHealth performs the health check for a given service using the factory pattern
func CheckServiceHealth(serviceType, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return models.ServiceHealth{
			Status:      "error",
			LastChecked: time.Now(),
			Message:     "URL is required",
		}, http.StatusBadRequest
	}

	// Get the appropriate service checker from the factory
	serviceChecker := serviceFactory.CreateService(serviceType)
	if serviceChecker == nil {
		log.Warn().Str("service_type", serviceType).Msg("No service checker found for type")
		return models.ServiceHealth{
			Status:       "error",
			ResponseTime: time.Since(startTime).Milliseconds(),
			LastChecked:  time.Now(),
			Message:      "Unsupported service type: " + serviceType,
		}, http.StatusBadRequest
	}

	// Use the service-specific implementation to check health
	health, statusCode := serviceChecker.CheckHealth(url, apiKey)
	return health, statusCode
}

func (h *HealthService) StartMonitoring(instanceID string, checkFn func(context.Context) (*HealthCheck, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If already monitoring this service, stop it first
	if cancel, exists := h.monitoredServices[instanceID]; exists {
		cancel()
	}

	// Create a new context for this service
	ctx, cancel := context.WithCancel(context.Background())
	h.monitoredServices[instanceID] = cancel

	// Perform initial health check
	health, err := checkFn(ctx)
	if err != nil {
		h.healthChecks[instanceID] = &HealthCheck{
			Status:      "error",
			Message:     err.Error(),
			LastChecked: time.Now(),
		}
	} else if health != nil {
		h.healthChecks[instanceID] = health
	}

	// Start monitoring in a goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				health, err := checkFn(ctx)
				h.mu.Lock()
				if err != nil {
					h.healthChecks[instanceID] = &HealthCheck{
						Status:      "error",
						Message:     err.Error(),
						LastChecked: time.Now(),
					}
				} else if health != nil {
					h.healthChecks[instanceID] = health
				}
				h.mu.Unlock()
			}
		}
	}()
}

func (h *HealthService) StopMonitoring(instanceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if cancel, exists := h.monitoredServices[instanceID]; exists {
		cancel()
		delete(h.monitoredServices, instanceID)
		delete(h.healthChecks, instanceID)
	}
}

func (h *HealthService) GetHealth(instanceID string) *HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, exists := h.healthChecks[instanceID]; exists {
		return health
	}
	return nil
}

func (h *HealthService) GetAllHealth() map[string]*HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Create a copy of the health checks map
	result := make(map[string]*HealthCheck, len(h.healthChecks))
	for k, v := range h.healthChecks {
		result[k] = v
	}
	return result
}

// Cleanup stops all monitoring
func (h *HealthService) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for instanceID, cancel := range h.monitoredServices {
		cancel()
		delete(h.monitoredServices, instanceID)
		delete(h.healthChecks, instanceID)
	}
}
