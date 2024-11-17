// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/resilience"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	tailscaleCacheDuration    = 60 * time.Second // Primary cache duration
	tailscaleStaleDataTimeout = 5 * time.Minute  // Stale data timeout
	devicesCachePrefix        = "tailscale:devices:"
	maxFailures               = 5
	resetTimeout              = time.Minute
)

type TailscaleHandler struct {
	db                *database.DB
	cache             cache.Store
	sf                singleflight.Group
	lastDevicesHash   map[string]string
	lastDevicesHashMu sync.Mutex
	circuitBreaker    *resilience.CircuitBreaker
}

func NewTailscaleHandler(db *database.DB, cache cache.Store) *TailscaleHandler {
	return &TailscaleHandler{
		db:              db,
		cache:           cache,
		lastDevicesHash: make(map[string]string),
		circuitBreaker:  resilience.NewCircuitBreaker(maxFailures, resetTimeout),
	}
}

func (h *TailscaleHandler) GetTailscaleDevices(c *gin.Context) {
	// Try both instanceId and direct apiKey validation
	instanceId := c.Query("instanceId")
	apiKey := c.Query("apiKey")

	var cacheKey string
	if apiKey != "" {
		cacheKey = devicesCachePrefix + "direct:" + apiKey[:8] // Use first 8 chars of API key for cache key
	} else if instanceId != "" {
		cacheKey = devicesCachePrefix + instanceId
	} else {
		// Try to get the first tailscale instance if no specific instance is requested
		services, err := h.db.GetAllServices(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("[Tailscale] Failed to fetch services")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch services"})
			return
		}

		for _, s := range services {
			if strings.HasPrefix(s.InstanceID, "tailscale") {
				instanceId = s.InstanceID
				cacheKey = devicesCachePrefix + instanceId
				break
			}
		}

		if instanceId == "" {
			log.Error().Msg("[Tailscale] No instance found")
			c.JSON(http.StatusBadRequest, gin.H{"error": "No Tailscale instance configured"})
			return
		}
	}

	ctx := context.Background()

	// Check if circuit breaker is open
	if h.circuitBreaker.IsOpen() {
		log.Warn().Msg("[Tailscale] Circuit breaker is open, serving stale data")
		if err := h.serveStaleData(c, cacheKey); err == nil {
			return
		}
		// If no stale data available, continue with the request
		log.Warn().Msg("[Tailscale] No stale data available, attempting fresh request")
	}

	// Try to get from cache first
	var response struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}
	err := h.cache.Get(ctx, cacheKey, &response)
	if err == nil {
		log.Debug().
			Int("deviceCount", len(response.Devices)).
			Msg("[Tailscale] Serving devices from cache")
		c.JSON(http.StatusOK, response)

		// Refresh cache in background using singleflight
		go func() {
			refreshKey := fmt.Sprintf("devices_refresh:%s", strings.TrimPrefix(cacheKey, devicesCachePrefix))
			_, _, _ = h.sf.Do(refreshKey, func() (interface{}, error) {
				h.refreshDevicesCache(instanceId, apiKey, cacheKey)
				return nil, nil
			})
		}()
		return
	}

	// If not in cache, fetch from service using singleflight
	sfKey := fmt.Sprintf("devices:%s", strings.TrimPrefix(cacheKey, devicesCachePrefix))
	devicesI, err, _ := h.sf.Do(sfKey, func() (interface{}, error) {
		var devices []tailscale.Device
		err := resilience.RetryWithBackoff(ctx, func() error {
			var fetchErr error
			devices, fetchErr = h.fetchAndCacheDevices(ctx, instanceId, apiKey, cacheKey)
			if fetchErr != nil {
				h.circuitBreaker.RecordFailure()
				return fetchErr
			}
			h.circuitBreaker.RecordSuccess()
			return nil
		})
		if err != nil {
			return nil, err
		}
		return devices, nil
	})

	if err != nil {
		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Msg("[Tailscale] Request timeout while fetching devices")
		} else {
			log.Error().Err(err).Msg("[Tailscale] Failed to fetch devices")
		}

		// Try to serve stale data on error
		if serveErr := h.serveStaleData(c, cacheKey); serveErr == nil {
			return
		}

		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	devices := devicesI.([]tailscale.Device)

	if devices == nil {
		// Return empty array instead of null
		log.Debug().Msg("[Tailscale] No devices found")
		c.JSON(http.StatusOK, gin.H{
			"devices": []interface{}{},
			"status":  "success",
		})
		return
	}

	// Use the new change detection method
	h.compareAndLogDeviceChanges(instanceId, devices)

	onlineCount := 0
	for _, device := range devices {
		if device.Online {
			onlineCount++
		}
	}

	log.Info().
		Int("total", len(devices)).
		Int("online", onlineCount).
		Msg("[Tailscale] Successfully retrieved and cached devices")

	response = struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}{
		Devices: devices,
		Status:  "success",
	}

	c.JSON(http.StatusOK, response)
}

func (h *TailscaleHandler) serveStaleData(c *gin.Context, cacheKey string) error {
	var response struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}

	staleCacheKey := cacheKey + ":stale"
	err := h.cache.Get(c.Request.Context(), staleCacheKey, &response)
	if err != nil {
		return err
	}

	log.Info().Msg("[Tailscale] Serving stale data")
	c.JSON(http.StatusOK, response)
	return nil
}

func (h *TailscaleHandler) fetchAndCacheDevices(ctx context.Context, instanceId, apiKey, cacheKey string) ([]tailscale.Device, error) {
	service := &tailscale.TailscaleService{}

	var devices []tailscale.Device
	var err error

	if apiKey != "" {
		devices, err = service.GetDevices(ctx, "", apiKey)
	} else {
		tailscaleConfig, err := h.db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: instanceId})
		if err != nil {
			return nil, fmt.Errorf("[Tailscale] failed to fetch configuration: %v", err)
		}

		if tailscaleConfig == nil {
			return nil, fmt.Errorf("[Tailscale] is not configured")
		}

		devices, err = service.GetDevices(ctx, "", tailscaleConfig.APIKey)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	// Cache the results
	response := struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}{
		Devices: devices,
		Status:  "success",
	}

	// Cache fresh data
	if err := h.cache.Set(ctx, cacheKey, response, tailscaleCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Tailscale] Failed to cache devices")
	}

	// Cache stale data with longer duration
	staleCacheKey := cacheKey + ":stale"
	if err := h.cache.Set(ctx, staleCacheKey, response, tailscaleStaleDataTimeout); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Tailscale] Failed to cache stale devices")
	}

	return devices, nil
}

func (h *TailscaleHandler) refreshDevicesCache(instanceId, apiKey, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	err := resilience.RetryWithBackoff(ctx, func() error {
		_, err := h.fetchAndCacheDevices(ctx, instanceId, apiKey, cacheKey)
		return err
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("[Tailscale] Failed to refresh devices cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("[Tailscale] Successfully refreshed devices cache")
}

func createDevicesHash(devices []tailscale.Device) string {
	if len(devices) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, device := range devices {
		// Include key device details that indicate meaningful changes
		fmt.Fprintf(&sb, "%s:%s:%t,",
			device.ID,
			device.LastSeen,
			device.Online,
		)
	}
	return sb.String()
}

func (h *TailscaleHandler) detectDeviceChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_devices"
	}

	oldDevices := strings.Split(oldHash, ",")
	newDevices := strings.Split(newHash, ",")

	if len(oldDevices) < len(newDevices) {
		return "device_added"
	} else if len(oldDevices) > len(newDevices) {
		return "device_removed"
	}

	return "device_state_changed"
}

func (h *TailscaleHandler) compareAndLogDeviceChanges(instanceId string, devices []tailscale.Device) {
	h.lastDevicesHashMu.Lock()
	defer h.lastDevicesHashMu.Unlock()

	currentHash := createDevicesHash(devices)
	lastHash := h.lastDevicesHash[instanceId]

	if currentHash != lastHash {
		// Detect specific changes
		changes := h.detectDeviceChanges(lastHash, currentHash)

		log.Info().
			Str("instanceId", instanceId).
			Int("total", len(devices)).
			Int("online", countOnlineDevices(devices)).
			Str("change", changes).
			Msg("Tailscale devices retrieved")

		h.lastDevicesHash[instanceId] = currentHash
	}
}

func countOnlineDevices(devices []tailscale.Device) int {
	onlineCount := 0
	for _, device := range devices {
		if device.Online {
			onlineCount++
		}
	}
	return onlineCount
}
