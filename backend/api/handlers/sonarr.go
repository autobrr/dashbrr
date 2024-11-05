package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/backend/database"
	"github.com/autobrr/dashbrr/backend/services/cache"
)

const (
	sonarrCacheDuration = 5 * time.Second
	sonarrQueuePrefix   = "sonarr:queue:"
	sonarrStatsPrefix   = "sonarr:stats:"
)

// SonarrQueueResponse represents the queue response from Sonarr API
type SonarrQueueResponse struct {
	Page          int           `json:"page"`
	PageSize      int           `json:"pageSize"`
	SortKey       string        `json:"sortKey"`
	SortDirection string        `json:"sortDirection"`
	TotalRecords  int           `json:"totalRecords"`
	Records       []QueueRecord `json:"records"`
}

// QueueRecord represents a record in the Sonarr queue
type QueueRecord struct {
	ID                    int    `json:"id"`
	Title                 string `json:"title"`
	Status                string `json:"status"`
	TimeLeft              string `json:"timeleft,omitempty"`
	Indexer               string `json:"indexer"`
	DownloadClient        string `json:"downloadClient"`
	CustomFormatScore     int    `json:"customFormatScore"`
	TrackedDownloadStatus string `json:"trackedDownloadStatus"`
	TrackedDownloadState  string `json:"trackedDownloadState"`
}

// SonarrStatsResponse represents the stats response from Sonarr API
type SonarrStatsResponse struct {
	MovieCount       int `json:"movieCount"`
	EpisodeCount     int `json:"episodeCount"`
	EpisodeFileCount int `json:"episodeFileCount"`
	Monitored        int `json:"monitored"`
	Unmonitored      int `json:"unmonitored"`
	QueuedCount      int `json:"queuedCount"`
	MissingCount     int `json:"missingCount"`
}

type SonarrHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewSonarrHandler(db *database.DB, cache *cache.Cache) *SonarrHandler {
	return &SonarrHandler{
		db:    db,
		cache: cache,
	}
}

func (h *SonarrHandler) GetQueue(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Sonarr instance
	if instanceId[:6] != "sonarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Sonarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrQueuePrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var queueResp SonarrQueueResponse
	err := h.cache.Get(ctx, cacheKey, &queueResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("totalRecords", queueResp.TotalRecords).
			Msg("Serving Sonarr queue from cache")
		c.JSON(http.StatusOK, queueResp)
		return
	}

	// If not in cache, fetch from service
	sonarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Sonarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Sonarr configuration"})
		return
	}

	if sonarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Sonarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Sonarr is not configured"})
		return
	}

	// Build Sonarr API URL
	apiURL := fmt.Sprintf("%s/api/v3/queue?apikey=%s", sonarrConfig.URL, sonarrConfig.APIKey)

	// Make request to Sonarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Sonarr queue")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Sonarr queue"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Sonarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Sonarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Sonarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Sonarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, queueResp, sonarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr queue")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("totalRecords", queueResp.TotalRecords).
		Msg("Successfully retrieved and cached Sonarr queue")

	c.JSON(http.StatusOK, queueResp)
}

func (h *SonarrHandler) GetStats(c *gin.Context) {
	instanceId := c.Query("instanceId")
	if instanceId == "" {
		log.Error().Msg("No instanceId provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// Verify this is a Sonarr instance
	if instanceId[:6] != "sonarr" {
		log.Error().Str("instanceId", instanceId).Msg("Invalid Sonarr instance ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Sonarr instance ID"})
		return
	}

	cacheKey := sonarrStatsPrefix + instanceId
	ctx := context.Background()

	// Try to get from cache first
	var statsResp SonarrStatsResponse
	err := h.cache.Get(ctx, cacheKey, &statsResp)
	if err == nil {
		log.Debug().
			Str("instanceId", instanceId).
			Int("monitored", statsResp.Monitored).
			Msg("Serving Sonarr stats from cache")
		c.JSON(http.StatusOK, statsResp)
		return
	}

	// If not in cache, fetch from service
	sonarrConfig, err := h.db.GetServiceByInstanceID(instanceId)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to get Sonarr configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Sonarr configuration"})
		return
	}

	if sonarrConfig == nil {
		log.Error().Str("instanceId", instanceId).Msg("Sonarr is not configured")
		c.JSON(http.StatusNotFound, gin.H{"error": "Sonarr is not configured"})
		return
	}

	// Build Sonarr API URL
	apiURL := fmt.Sprintf("%s/api/v3/system/status?apikey=%s", sonarrConfig.URL, sonarrConfig.APIKey)

	// Make request to Sonarr
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to fetch Sonarr stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Sonarr stats"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("instanceId", instanceId).
			Int("statusCode", resp.StatusCode).
			Msg("Sonarr API returned non-200 status")
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Sonarr API returned status: %d", resp.StatusCode)})
		return
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		log.Error().Err(err).Str("instanceId", instanceId).Msg("Failed to parse Sonarr response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Sonarr response"})
		return
	}

	// Cache the results
	if err := h.cache.Set(ctx, cacheKey, statsResp, sonarrCacheDuration); err != nil {
		log.Warn().
			Err(err).
			Str("instanceId", instanceId).
			Msg("Failed to cache Sonarr stats")
	}

	log.Debug().
		Str("instanceId", instanceId).
		Int("monitored", statsResp.Monitored).
		Msg("Successfully retrieved and cached Sonarr stats")

	c.JSON(http.StatusOK, statsResp)
}
