// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/services/tailscale"
	"github.com/autobrr/dashbrr/internal/types"
)

const (
	tailscaleCacheDuration = 60 * time.Second // Longer cache for Tailscale as it changes less frequently
	devicesCachePrefix     = "tailscale:devices:"
)

type TailscaleHandler struct {
	db    *database.DB
	cache cache.Store
}

func NewTailscaleHandler(db *database.DB, cache cache.Store) *TailscaleHandler {
	return &TailscaleHandler{
		db:    db,
		cache: cache,
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
		services, err := h.db.GetAllServices()
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch services")
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
			log.Error().Msg("No Tailscale instance found")
			c.JSON(http.StatusBadRequest, gin.H{"error": "No Tailscale instance configured"})
			return
		}
	}

	ctx := context.Background()

	// Try to get from cache first
	var response struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}
	err := h.cache.Get(ctx, cacheKey, &response)
	if err == nil {
		log.Debug().
			Int("deviceCount", len(response.Devices)).
			Msg("Serving Tailscale devices from cache")
		c.JSON(http.StatusOK, response)

		// Refresh cache in background if needed
		go h.refreshDevicesCache(instanceId, apiKey, cacheKey)
		return
	}

	// If not in cache, fetch from service
	devices, err := h.fetchAndCacheDevices(ctx, instanceId, apiKey, cacheKey)
	if err != nil {
		status := http.StatusInternalServerError
		if err == context.DeadlineExceeded || err == context.Canceled {
			status = http.StatusGatewayTimeout
			log.Error().Err(err).Msg("Request timeout while fetching Tailscale devices")
		} else {
			log.Error().Err(err).Msg("Failed to fetch Tailscale devices")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if devices == nil {
		// Return empty array instead of null
		log.Debug().Msg("No Tailscale devices found")
		c.JSON(http.StatusOK, gin.H{
			"devices": []interface{}{},
			"status":  "success",
		})
		return
	}

	onlineCount := 0
	for _, device := range devices {
		if device.Online {
			onlineCount++
		}
	}

	log.Info().
		Int("total", len(devices)).
		Int("online", onlineCount).
		Msg("Successfully retrieved and cached Tailscale devices")

	response = struct {
		Devices []tailscale.Device `json:"devices"`
		Status  string             `json:"status"`
	}{
		Devices: devices,
		Status:  "success",
	}

	c.JSON(http.StatusOK, response)
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
			return nil, fmt.Errorf("failed to fetch tailscale configuration: %v", err)
		}

		if tailscaleConfig == nil {
			return nil, fmt.Errorf("tailscale is not configured")
		}

		devices, err = service.GetDevices(ctx, "", tailscaleConfig.APIKey)
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

	if err := h.cache.Set(ctx, cacheKey, response, tailscaleCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Tailscale devices")
	}

	return devices, nil
}

func (h *TailscaleHandler) refreshDevicesCache(instanceId, apiKey, cacheKey string) {
	// Add a small delay to prevent immediate refresh
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	_, err := h.fetchAndCacheDevices(ctx, instanceId, apiKey, cacheKey)
	if err != nil {
		log.Error().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to refresh Tailscale devices cache")
		return
	}

	log.Debug().
		Str("instanceId", instanceId).
		Msg("Successfully refreshed Tailscale devices cache")
}
