// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/general"
)

// generalEngine is the minimal surface GeneralHandler needs from
// general.Engine (item D2). Defining it locally keeps this package's
// dependency on the engine to exactly the methods it calls, and lets tests
// substitute a stub instead of driving real HTTP requests.
type generalEngine interface {
	CheckHealth(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (models.ServiceHealth, int)
	FetchStats(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (map[string]general.StatValue, error)
	RunAction(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, actionID string) (general.ActionResult, error)
}

// newGeneralEngine returns the concrete D2 engine, addressed through the
// generalEngine interface above.
func newGeneralEngine() generalEngine {
	service := general.NewGeneralService().(*general.GeneralService)
	return &service.Engine
}

// GeneralHandler serves the config/action/test endpoints for "general"
// (user-defined custom) service instances.
type GeneralHandler struct {
	db     *database.DB
	engine generalEngine
}

func NewGeneralHandler(db *database.DB) *GeneralHandler {
	return &GeneralHandler{db: db, engine: newGeneralEngine()}
}

// generalInstanceType reports whether instanceID belongs to a "general"
// service, without requiring the row to already exist.
func generalInstanceType(instanceID string) bool {
	serviceType, ok := models.ServiceTypeFromInstanceID(instanceID)
	return ok && serviceType == "general"
}

// GetConfig returns the redacted CustomServiceConfig for instanceId.
func (h *GeneralHandler) GetConfig(c *gin.Context) {
	instanceID := c.Param("instanceId")
	if !generalInstanceType(instanceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a general service instance"})
		return
	}

	svc, err := findServiceConfig(c.Request.Context(), h.db, instanceID)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[General] Failed to load service configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load service configuration"})
		return
	}
	if svc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}

	c.JSON(http.StatusOK, redactedGeneralConfig(svc.Config))
}

// PutConfig validates and persists a new CustomServiceConfig for instanceId.
// Blank secret fields (auth password/token, login body) in the incoming
// payload keep their previously stored values, matching the write-only
// secret handling used elsewhere in the settings API.
func (h *GeneralHandler) PutConfig(c *gin.Context) {
	instanceID := c.Param("instanceId")
	if !generalInstanceType(instanceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a general service instance"})
		return
	}

	var cfg models.CustomServiceConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []string{"invalid request body: " + err.Error()}})
		return
	}

	ctx := c.Request.Context()
	existing, err := findServiceConfig(ctx, h.db, instanceID)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[General] Failed to load service configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load service configuration"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}

	mergeGeneralSecrets(&cfg, existing.Config)

	if err := cfg.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []string{err.Error()}})
		return
	}

	existing.Config = &cfg
	if err := h.db.UpdateService(ctx, existing); err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[General] Failed to persist service configuration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save service configuration"})
		return
	}

	log.Info().Str("instanceId", instanceID).Msg("[General] Saved service configuration")
	c.JSON(http.StatusOK, redactedGeneralConfig(existing.Config))
}

// RunAction runs a single configured action for instanceId. Actions marked
// Confirm require the caller to send X-Confirm: yes, otherwise the request
// is rejected with 428 Precondition Required.
func (h *GeneralHandler) RunAction(c *gin.Context) {
	instanceID := c.Param("instanceId")
	if !generalInstanceType(instanceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a general service instance"})
		return
	}
	actionID := c.Param("actionId")

	ctx := c.Request.Context()
	svc, err := requireServiceConfig(ctx, h.db, instanceID, "general")
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Msg("[General] Failed to load service configuration")
		c.JSON(statusFromGeneralError(err), gin.H{"error": err.Error()})
		return
	}

	action := findGeneralAction(svc.Config, actionID)
	if action == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown action: " + actionID})
		return
	}

	if action.Confirm && c.GetHeader("X-Confirm") != "yes" {
		c.JSON(http.StatusPreconditionRequired, gin.H{"error": "this action requires confirmation", "confirm": true})
		return
	}

	result, err := h.engine.RunAction(ctx, svc.URL, svc.APIKey, svc.Config, actionID)
	if err != nil {
		log.Error().Err(err).Str("instanceId", instanceID).Str("actionId", actionID).Msg("[General] Action failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": result.Status, "body": result.Body})
}

type generalTestRequest struct {
	URL    string                      `json:"url"`
	APIKey string                      `json:"apiKey"` //nolint:gosec // struct field name, not a credential
	Config *models.CustomServiceConfig `json:"config"`
}

type generalTestResponse struct {
	Status  string                       `json:"status"`
	Version string                       `json:"version,omitempty"`
	Stats   map[string]general.StatValue `json:"stats,omitempty"`
	Error   string                       `json:"error,omitempty"`
}

// Test runs one health check and one stats fetch against an ad hoc
// definition, without persisting anything. Used by the UI "Test" button and
// the CLI's `service generic test` command.
func (h *GeneralHandler) Test(c *gin.Context) {
	var req generalTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []string{"invalid request body: " + err.Error()}})
		return
	}

	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []string{"url is required"}})
		return
	}

	if err := req.Config.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []string{err.Error()}})
		return
	}

	ctx := c.Request.Context()
	health, _ := h.engine.CheckHealth(ctx, req.URL, req.APIKey, req.Config)

	resp := generalTestResponse{Status: health.Status, Version: health.Version}

	stats, err := h.engine.FetchStats(ctx, req.URL, req.APIKey, req.Config)
	if err != nil {
		resp.Error = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Stats = stats

	c.JSON(http.StatusOK, resp)
}

// redactedGeneralConfig returns a safe-to-return copy of cfg, or an empty
// (zero-value) config when none is configured yet.
func redactedGeneralConfig(cfg *models.CustomServiceConfig) models.CustomServiceConfig {
	if cfg == nil {
		return models.CustomServiceConfig{}
	}
	return cfg.Redacted()
}

// mergeGeneralSecrets fills blank secret fields on next from existing so a
// PUT that omits them (because GetConfig never returns them) does not wipe
// out previously stored credentials.
func mergeGeneralSecrets(next, existing *models.CustomServiceConfig) {
	if next == nil || existing == nil {
		return
	}

	if next.Auth != nil && existing.Auth != nil {
		if next.Auth.Password == "" {
			next.Auth.Password = existing.Auth.Password
		}
		if next.Auth.Token == "" {
			next.Auth.Token = existing.Auth.Token
		}
	}

	if next.Login != nil && existing.Login != nil && next.Login.Body == "" {
		next.Login.Body = existing.Login.Body
	}
}

func findGeneralAction(cfg *models.CustomServiceConfig, actionID string) *models.CustomActionConfig {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Actions {
		if cfg.Actions[i].ID == actionID {
			return &cfg.Actions[i]
		}
	}
	return nil
}

func statusFromGeneralError(err error) int {
	if errors.Is(err, ErrServiceNotConfigured) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
