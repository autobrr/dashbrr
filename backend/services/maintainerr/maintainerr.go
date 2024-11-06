// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package maintainerr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/autobrr/dashbrr/backend/models"
	"github.com/autobrr/dashbrr/backend/services/base"
)

type MaintainerrService struct {
	base.BaseService
}

type StatusResponse struct {
	Version         string `json:"version"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

type Media struct {
	ID           int       `json:"id"`
	CollectionID int       `json:"collectionId"`
	PlexID       int       `json:"plexId"`
	TmdbID       int       `json:"tmdbId"`
	AddDate      time.Time `json:"addDate"`
	ImagePath    string    `json:"image_path"`
	IsManual     bool      `json:"isManual"`
}

type Collection struct {
	ID                int     `json:"id"`
	PlexID            int     `json:"plexId"`
	LibraryID         int     `json:"libraryId"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	IsActive          bool    `json:"isActive"`
	ArrAction         int     `json:"arrAction"`
	VisibleOnHome     bool    `json:"visibleOnHome"`
	DeleteAfterDays   int     `json:"deleteAfterDays"`
	ManualCollection  bool    `json:"manualCollection"`
	ListExclusions    bool    `json:"listExclusions"`
	ForceOverseerr    bool    `json:"forceOverseerr"`
	Type              int     `json:"type"`
	KeepLogsForMonths int     `json:"keepLogsForMonths"`
	AddDate           string  `json:"addDate"`
	Media             []Media `json:"media"`
}

func init() {
	models.NewMaintainerrService = func() models.ServiceHealthChecker {
		return &MaintainerrService{}
	}
}

func (s *MaintainerrService) GetHealthEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/api/app/status", baseURL)
}

func (s *MaintainerrService) getVersion(ctx context.Context, url string) (string, error) {
	if version := s.GetVersionFromCache(url); version != "" {
		return version, nil
	}

	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.ReadBody(resp)
	if err != nil {
		return "", err
	}

	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return "", err
	}

	if err := s.CacheVersion(url, statusResponse.Version, time.Hour); err != nil {
		fmt.Printf("Failed to cache version: %v\n", err)
	}

	return statusResponse.Version, nil
}

func (s *MaintainerrService) CheckHealth(url, apiKey string) (models.ServiceHealth, int) {
	startTime := time.Now()

	if url == "" {
		return s.CreateHealthResponse(startTime, "error", "URL is required"), http.StatusBadRequest
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	versionChan := make(chan string, 1)
	go func() {
		version, err := s.getVersion(ctx, url)
		if err != nil {
			versionChan <- ""
			return
		}
		versionChan <- version
	}()

	healthEndpoint := s.GetHealthEndpoint(url)
	resp, err := s.MakeRequestWithContext(ctx, healthEndpoint, "", nil)
	if err != nil {
		return s.CreateHealthResponse(startTime, "offline", "Failed to connect: "+err.Error()), http.StatusOK
	}
	defer resp.Body.Close()

	responseTime, _ := time.ParseDuration(resp.Header.Get("X-Response-Time") + "ms")

	body, err := s.ReadBody(resp)
	if err != nil {
		return s.CreateHealthResponse(startTime, "warning", "Failed to read response: "+err.Error()), http.StatusOK
	}

	if resp.StatusCode >= 400 {
		return s.CreateHealthResponse(startTime, "error", fmt.Sprintf("Server returned error: %d", resp.StatusCode)), http.StatusOK
	}

	var statusResponse StatusResponse
	if err := json.Unmarshal(body, &statusResponse); err != nil {
		return s.CreateHealthResponse(startTime, "warning", "Failed to parse status response"), http.StatusOK
	}

	var version string
	select {
	case v := <-versionChan:
		version = v
	case <-time.After(2 * time.Second):
	}

	extras := map[string]interface{}{
		"version":         version,
		"updateAvailable": statusResponse.UpdateAvailable,
		"responseTime":    responseTime.Milliseconds(),
	}

	return s.CreateHealthResponse(startTime, "online", "Healthy", extras), http.StatusOK
}

func (s *MaintainerrService) GetCollections(url, apiKey string) ([]Collection, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(url, "/")
	endpoint := fmt.Sprintf("%s/api/collections", baseURL)

	resp, err := s.MakeRequestWithContext(ctx, endpoint, apiKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned error %d", resp.StatusCode)
	}

	body, err := s.ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var collections []Collection
	if err := json.Unmarshal(body, &collections); err != nil {
		var singleCollection Collection
		if err := json.Unmarshal(body, &singleCollection); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		if singleCollection.IsActive {
			collections = []Collection{singleCollection}
		} else {
			collections = []Collection{}
		}
	}

	activeCollections := make([]Collection, 0)
	for _, collection := range collections {
		if collection.IsActive {
			activeCollections = append(activeCollections, collection)
		}
	}

	return activeCollections, nil
}
