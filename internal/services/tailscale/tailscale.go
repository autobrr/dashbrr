// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
)

type TailscaleService struct {
	core.ServiceCore
}

type Device struct {
	Name            string   `json:"name"`
	ID              string   `json:"id"`
	IPAddress       string   `json:"ipAddress"`
	LastSeen        string   `json:"lastSeen"`
	Online          bool     `json:"online"`
	OS              string   `json:"os"`
	DeviceType      string   `json:"deviceType"`
	ClientVersion   string   `json:"clientVersion"`
	UpdateAvailable bool     `json:"updateAvailable"`
	Tags            []string `json:"tags"`
}

type TailscaleAPIResponse struct {
	Devices []struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		Addresses          []string `json:"addresses"`
		LastSeen           string   `json:"lastSeen"`
		OS                 string   `json:"os"`
		Hostname           string   `json:"hostname"`
		Authorized         bool     `json:"authorized"`
		ClientVersion      string   `json:"clientVersion"`
		UpdateAvailable    bool     `json:"updateAvailable"`
		Tags               []string `json:"tags"`
		ClientConnectivity *struct {
			Endpoints []string `json:"endpoints"`
		} `json:"clientConnectivity,omitempty"`
	} `json:"devices"`
}

func init() {
	models.NewTailscaleService = NewTailscaleService
}

func NewTailscaleService() models.ServiceHealthChecker {
	service := &TailscaleService{}
	service.Type = "tailscale"
	service.DisplayName = "Tailscale"
	service.Description = "Manage and monitor your Tailscale network"
	service.DefaultURL = "https://api.tailscale.com"
	service.HealthEndpoint = "/api/v2/tailnet/-/devices"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *TailscaleService) getDevicesWithContext(ctx context.Context, apiKey string) (*TailscaleAPIResponse, time.Duration, error) {
	startTime := time.Now()

	if !strings.HasPrefix(apiKey, "tskey-api-") {
		return nil, 0, fmt.Errorf("invalid API key format. Must start with 'tskey-api-'")
	}

	// Use the correct API endpoint with the tailnet parameter
	devicesURL := "https://api.tailscale.com/api/v2/tailnet/-/devices"

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Accept":        "application/json",
	}

	resp, err := s.MakeRequestWithContext(ctx, devicesURL, "", headers)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, responseTime, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, responseTime, fmt.Errorf("API request failed (Status %d): %s", resp.StatusCode, string(body))
	}

	var apiResponse TailscaleAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, responseTime, fmt.Errorf("failed to decode response: %v", err)
	}

	return &apiResponse, responseTime, nil
}

func (s *TailscaleService) CheckHealth(ctx context.Context, _ string, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if apiKey == "" {
		return s.CreateHealthResponse(startTime, "error", "Service not configured: missing API key"), http.StatusBadRequest
	}

	// Create a child context with timeout if needed
	healthCtx, cancel := context.WithTimeout(ctx, core.DefaultTimeout)
	defer cancel()

	versionChan := make(chan string, 1)
	errChan := make(chan error, 1)
	go func() {
		apiResponse, _, err := s.getDevicesWithContext(healthCtx, apiKey)
		if err != nil {
			errChan <- err
			versionChan <- ""
			return
		}

		var version string
		if len(apiResponse.Devices) > 0 {
			version = apiResponse.Devices[0].ClientVersion
		}
		versionChan <- version
		errChan <- nil
	}()

	apiResponse, responseTime, err := s.getDevicesWithContext(healthCtx, apiKey)
	if err != nil {
		return s.CreateHealthResponse(startTime, "error", err.Error()), http.StatusServiceUnavailable
	}

	var version string
	var versionErr error
	select {
	case version = <-versionChan:
		versionErr = <-errChan
	case <-healthCtx.Done():
		versionErr = healthCtx.Err()
	}

	onlineCount := 0
	for _, device := range apiResponse.Devices {
		if isDeviceOnline(device.LastSeen) {
			onlineCount++
		}
	}

	extras := map[string]interface{}{
		"responseTime": responseTime.Milliseconds(),
	}

	if version != "" {
		extras["version"] = version
	}
	if versionErr != nil {
		extras["versionError"] = versionErr.Error()
	}

	return s.CreateHealthResponse(startTime, "online", fmt.Sprintf("%d devices online", onlineCount), extras), http.StatusOK
}

func isDeviceOnline(lastSeen string) bool {
	lastSeenTime, err := time.Parse(time.RFC3339, lastSeen)
	if err != nil {
		return false
	}

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	return lastSeenTime.After(fiveMinutesAgo)
}

func (s *TailscaleService) GetDevices(ctx context.Context, _ string, apiKey string) ([]Device, error) {
	apiResponse, _, err := s.getDevicesWithContext(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	var devices []Device
	for _, d := range apiResponse.Devices {
		var ipAddress string
		if len(d.Addresses) > 0 {
			ipAddress = d.Addresses[0]
		}

		online := isDeviceOnline(d.LastSeen)

		devices = append(devices, Device{
			Name:            d.Name,
			ID:              d.ID,
			IPAddress:       ipAddress,
			LastSeen:        d.LastSeen,
			Online:          online,
			OS:              d.OS,
			DeviceType:      d.OS,
			ClientVersion:   d.ClientVersion,
			UpdateAvailable: d.UpdateAvailable,
			Tags:            d.Tags,
		})
	}

	return devices, nil
}
