// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
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
)

const (
	tailscaleCacheDuration    = 60 * time.Second // Primary cache duration
	tailscaleStaleDataTimeout = 5 * time.Minute  // Stale data timeout
	devicesCachePrefix        = "tailscale:devices:"
	maxFailures               = 5
	resetTimeout              = time.Minute
)

type tailscaleDevicesResponse struct {
	Devices []tailscale.Device `json:"devices"`
	Status  string             `json:"status"`
}

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
		keyPrefix := apiKey
		if len(keyPrefix) > 8 {
			keyPrefix = keyPrefix[:8]
		}
		cacheKey = devicesCachePrefix + "direct:" + keyPrefix // Use first 8 chars of API key for cache key
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

	ctx := c.Request.Context()
	hashKey := strings.TrimPrefix(cacheKey, devicesCachePrefix)

	sfKey := fmt.Sprintf("devices:%s", hashKey)
	response, err := FetchWithSWRCache(ctx, SWRCacheOptions[tailscaleDevicesResponse]{
		Store:           h.cache,
		Key:             cacheKey,
		FreshTTL:        tailscaleCacheDuration,
		StaleTTL:        tailscaleStaleDataTimeout,
		CircuitBreaker:  h.circuitBreaker,
		Singleflight:    &h.sf,
		SingleflightKey: sfKey,
		Fetch: func() (tailscaleDevicesResponse, error) {
			service := &tailscale.TailscaleService{}

			var devices []tailscale.Device
			var err error
			if apiKey != "" {
				devices, err = service.GetDevices(ctx, "", apiKey)
			} else {
				tailscaleConfig, err := requireServiceConfig(ctx, h.db, instanceId, "tailscale")
				if err != nil {
					return tailscaleDevicesResponse{}, err
				}
				devices, err = service.GetDevices(ctx, "", tailscaleConfig.APIKey)
			}
			if err != nil {
				return tailscaleDevicesResponse{}, err
			}
			if devices == nil {
				devices = []tailscale.Device{}
			}
			return tailscaleDevicesResponse{
				Devices: devices,
				Status:  "success",
			}, nil
		},
	})

	if err != nil {
		if errors.Is(err, ErrServiceNotConfigured) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Msg("[Tailscale] Request timeout while fetching devices")
		} else {
			log.Error().Err(err).Msg("[Tailscale] Failed to fetch devices")
		}

		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	devices := response.Devices

	// Use the new change detection method
	h.compareAndLogDeviceChanges(hashKey, devices)

	onlineCount := countOnlineDevices(devices)

	log.Info().
		Int("total", len(devices)).
		Int("online", onlineCount).
		Msg("[Tailscale] Successfully retrieved and cached devices")

	c.JSON(http.StatusOK, response)
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
