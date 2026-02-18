// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/core"
	"github.com/autobrr/dashbrr/internal/types"
)

const quiTransferParallelLimit = 4

type QuiService struct {
	core.ServiceCore
}

func init() {
	models.NewQuiService = NewQuiService
}

func NewQuiService() models.ServiceHealthChecker {
	service := &QuiService{}
	service.Type = "qui"
	service.DisplayName = "Qui"
	service.Description = "Monitor qui and connected qBittorrent instances"
	service.DefaultURL = "http://localhost:7476"
	service.HealthEndpoint = "/health"
	service.SetTimeout(core.DefaultTimeout)
	return service
}

func (s *QuiService) requestJSON(ctx context.Context, requestURL, apiKey string, includeAuth bool, target any) (int, error) {
	headers := map[string]string{}
	if includeAuth {
		headers["X-API-Key"] = apiKey
	}

	resp, err := s.DoRequest(ctx, http.MethodGet, requestURL, headers, nil)
	if err != nil {
		return 0, err
	}

	statusCode := resp.StatusCode
	body, err := s.ReadBody(resp)
	if err != nil {
		return statusCode, err
	}

	if statusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(statusCode)
		}
		return statusCode, fmt.Errorf("status %d: %s", statusCode, msg)
	}

	if target == nil {
		return statusCode, nil
	}

	if err := json.Unmarshal(body, target); err != nil {
		return statusCode, fmt.Errorf("failed to parse response: %w", err)
	}

	return statusCode, nil
}

func (s *QuiService) getInstances(ctx context.Context, url, apiKey string) ([]types.QuiInstance, int, error) {
	var instances []types.QuiInstance
	statusCode, err := s.requestJSON(
		ctx,
		fmt.Sprintf("%s/api/instances", strings.TrimRight(url, "/")),
		apiKey,
		true,
		&instances,
	)
	if instances == nil {
		instances = []types.QuiInstance{}
	}
	return instances, statusCode, err
}

func (s *QuiService) GetInstances(ctx context.Context, url, apiKey string) ([]types.QuiInstance, error) {
	instances, _, err := s.getInstances(ctx, url, apiKey)
	return instances, err
}

func (s *QuiService) GetTransferInfo(ctx context.Context, url, apiKey string, instanceID int) (*types.QuiTransferInfo, error) {
	var info types.QuiTransferInfo
	_, err := s.requestJSON(
		ctx,
		fmt.Sprintf("%s/api/instances/%d/transfer-info", strings.TrimRight(url, "/"), instanceID),
		apiKey,
		true,
		&info,
	)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *QuiService) GetCrossSeedStatus(ctx context.Context, url, apiKey string) (*types.QuiCrossSeedStatus, error) {
	var status types.QuiCrossSeedStatus
	_, err := s.requestJSON(
		ctx,
		fmt.Sprintf("%s/api/cross-seed/status", strings.TrimRight(url, "/")),
		apiKey,
		true,
		&status,
	)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func summarizeQuiInstances(instances []types.QuiInstance) (active int, connected int, withCredentialErrors int) {
	for _, instance := range instances {
		if !instance.IsActive {
			continue
		}

		active++
		if instance.Connected {
			connected++
		}
		if instance.HasDecryptionError {
			withCredentialErrors++
		}
	}
	return active, connected, withCredentialErrors
}

func (s *QuiService) GetAggregatedTransferInfo(ctx context.Context, url, apiKey string, instances []types.QuiInstance) (types.QuiTransferSummary, []types.QuiInstanceTransfer) {
	summary := types.QuiTransferSummary{
		TotalInstances: len(instances),
	}

	transfers := make([]types.QuiInstanceTransfer, 0, len(instances))
	for _, instance := range instances {
		if !instance.IsActive {
			continue
		}

		summary.ActiveInstances++
		if instance.Connected {
			summary.ConnectedInstances++
		}

		transfers = append(transfers, types.QuiInstanceTransfer{
			InstanceID:       instance.ID,
			Name:             instance.Name,
			Active:           instance.IsActive,
			Connected:        instance.Connected,
			ConnectionStatus: instance.ConnectionStatus,
		})
	}

	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(quiTransferParallelLimit)

	for idx := range transfers {
		if !transfers[idx].Connected {
			continue
		}

		idx := idx
		instanceID := transfers[idx].InstanceID
		group.Go(func() error {
			info, err := s.GetTransferInfo(groupCtx, url, apiKey, instanceID)
			if err != nil || info == nil {
				return nil
			}

			mu.Lock()
			transfers[idx].Downloaded = info.Downloaded
			transfers[idx].Uploaded = info.Uploaded
			transfers[idx].DownloadSpeed = info.DownloadSpeed
			transfers[idx].UploadSpeed = info.UploadSpeed
			transfers[idx].DHTNodes = info.DHTNodes

			summary.Downloaded += info.Downloaded
			summary.Uploaded += info.Uploaded
			summary.DownloadSpeed += info.DownloadSpeed
			summary.UploadSpeed += info.UploadSpeed
			summary.DHTNodes += info.DHTNodes
			mu.Unlock()

			return nil
		})
	}

	_ = group.Wait()
	return summary, transfers
}

func (s *QuiService) CheckHealth(ctx context.Context, url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if strings.TrimSpace(url) == "" {
		return s.CreateHealthResponse(startTime, "error", "Service not configured: missing URL"), http.StatusBadRequest
	}
	if strings.TrimSpace(apiKey) == "" {
		return s.CreateHealthResponse(startTime, "error", "Service not configured: missing API key"), http.StatusBadRequest
	}

	baseURL := strings.TrimRight(url, "/")

	var healthResp types.QuiHealthResponse
	if _, err := s.requestJSON(ctx, fmt.Sprintf("%s/health", baseURL), "", false, &healthResp); err != nil {
		return s.CreateHealthResponse(startTime, "offline", fmt.Sprintf("Failed to connect to qui: %v", err)), http.StatusOK
	}

	instances, statusCode, err := s.getInstances(ctx, baseURL, apiKey)
	if err != nil {
		switch statusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return s.CreateHealthResponse(startTime, "error", "Invalid API key"), statusCode
		case http.StatusNotFound:
			return s.CreateHealthResponse(startTime, "warning", "Connected to qui but required API endpoint was not found"), http.StatusOK
		default:
			return s.CreateHealthResponse(startTime, "warning", fmt.Sprintf("Connected to qui but failed to query instances: %v", err)), http.StatusOK
		}
	}

	active, connected, withCredentialErrors := summarizeQuiInstances(instances)
	status := "online"
	message := fmt.Sprintf("%d/%d active instances connected", connected, active)

	if len(instances) == 0 {
		status = "warning"
		message = "Connected to qui, but no instances are configured"
	} else if active == 0 {
		status = "warning"
		message = "Connected to qui, but no active instances are enabled"
	} else if connected < active {
		status = "warning"
	}

	if withCredentialErrors > 0 {
		status = "warning"
		message = fmt.Sprintf("%s (%d with credential errors)", message, withCredentialErrors)
	}

	return s.CreateHealthResponse(
		startTime,
		status,
		message,
		map[string]interface{}{
			"responseTime": time.Since(startTime).Milliseconds(),
			"stats": map[string]interface{}{
				"qui": map[string]interface{}{
					"instances": instances,
				},
			},
			"details": map[string]interface{}{
				"qui": map[string]interface{}{
					"summary": types.QuiTransferSummary{
						TotalInstances:     len(instances),
						ActiveInstances:    active,
						ConnectedInstances: connected,
					},
				},
			},
		},
	), http.StatusOK
}
