// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/general"
	"github.com/autobrr/dashbrr/internal/types"
)

// stubGeneralEngine is a generalEngine implementation whose behaviour is
// controlled per test, so handler tests can exercise the confirm-header and
// persistence logic without making real HTTP requests to an upstream
// service.
type stubGeneralEngine struct {
	checkHealth func(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (models.ServiceHealth, int)
	fetchStats  func(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (map[string]general.StatValue, error)
	runAction   func(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, actionID string) (general.ActionResult, error)
}

func (s *stubGeneralEngine) CheckHealth(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (models.ServiceHealth, int) {
	if s.checkHealth != nil {
		return s.checkHealth(ctx, rawURL, apiKey, cfg)
	}
	return models.ServiceHealth{Status: "online"}, http.StatusOK
}

func (s *stubGeneralEngine) FetchStats(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig) (map[string]general.StatValue, error) {
	if s.fetchStats != nil {
		return s.fetchStats(ctx, rawURL, apiKey, cfg)
	}
	return map[string]general.StatValue{}, nil
}

func (s *stubGeneralEngine) RunAction(ctx context.Context, rawURL, apiKey string, cfg *models.CustomServiceConfig, actionID string) (general.ActionResult, error) {
	if s.runAction != nil {
		return s.runAction(ctx, rawURL, apiKey, cfg, actionID)
	}
	return general.ActionResult{Status: http.StatusOK, Body: "{}"}, nil
}

func newGeneralTestRouter(t *testing.T, handler *GeneralHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	generalGroup := r.Group("/api/general")
	{
		generalGroup.POST("/test", handler.Test)
		instance := generalGroup.Group("/:instanceId")
		{
			instance.GET("/config", handler.GetConfig)
			instance.PUT("/config", handler.PutConfig)
			instance.POST("/actions/:actionId", handler.RunAction)
		}
	}
	return r
}

func doRequest(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// PUT .../config with a structurally invalid definition must fail
// validation with 400 and an error list, and must not touch the database.
func TestGeneralHandler_PutConfig_ValidationError(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	// health.path is required whenever health is present.
	body := `{"health":{"statusPath":"status"}}`
	rec := doRequest(t, r, http.MethodPut, "/api/general/general-1/config", body, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected a non-empty error list")
	}

	stored, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "general-1"})
	if err != nil {
		t.Fatalf("find service: %v", err)
	}
	if stored.Config != nil {
		t.Fatalf("expected config to remain unset after a validation failure, got %+v", stored.Config)
	}
}

// PUT .../config with a valid definition persists it, and GET returns the
// redacted copy back (secrets blanked, but structurally present).
func TestGeneralHandler_PutConfig_PersistsAndRedacts(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	body := `{
		"auth": {"mode": "bearer", "token": "super-secret"},
		"health": {"path": "/health", "statusPath": "status", "okValues": ["ok"]},
		"stats": [{"label": "Queue", "path": "queue.length"}]
	}`
	putRec := doRequest(t, r, http.MethodPut, "/api/general/general-1/config", body, nil)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body=%s", putRec.Code, http.StatusOK, putRec.Body.String())
	}

	var putResp models.CustomServiceConfig
	if err := json.Unmarshal(putRec.Body.Bytes(), &putResp); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if putResp.Auth == nil || putResp.Auth.Token != "" {
		t.Fatalf("expected PUT response to redact the auth token, got %+v", putResp.Auth)
	}

	stored, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "general-1"})
	if err != nil {
		t.Fatalf("find service: %v", err)
	}
	if stored.Config == nil {
		t.Fatal("expected config to be persisted")
	}
	if stored.Config.Auth == nil || stored.Config.Auth.Token != "super-secret" {
		t.Fatalf("expected the stored (unredacted) config to keep the token, got %+v", stored.Config.Auth)
	}

	getRec := doRequest(t, r, http.MethodGet, "/api/general/general-1/config", "", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var getResp models.CustomServiceConfig
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getResp.Auth == nil || getResp.Auth.Token != "" {
		t.Fatalf("expected GET response to redact the auth token, got %+v", getResp.Auth)
	}
	if getResp.Health == nil || getResp.Health.Path != "/health" {
		t.Fatalf("expected non-secret fields to round-trip, got %+v", getResp.Health)
	}
}

// A PUT that omits a secret field (auth.token) must keep the previously
// stored value rather than wiping it out, matching the settings API's
// write-only secret handling.
func TestGeneralHandler_PutConfig_KeepsSecretWhenBlank(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
		Config: &models.CustomServiceConfig{
			Auth: &models.CustomAuthConfig{Mode: "bearer", Token: "original-token"},
		},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	// Same auth section, but the token field is blank - as it would be when
	// the UI round-trips a redacted GetConfig response back through PUT.
	body := `{"auth": {"mode": "bearer"}}`
	rec := doRequest(t, r, http.MethodPut, "/api/general/general-1/config", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	stored, err := db.FindServiceBy(ctx, types.FindServiceParams{InstanceID: "general-1"})
	if err != nil {
		t.Fatalf("find service: %v", err)
	}
	if stored.Config == nil || stored.Config.Auth == nil || stored.Config.Auth.Token != "original-token" {
		t.Fatalf("expected the original token to be preserved, got %+v", stored.Config)
	}
}

// An action with confirm:true must be rejected with 428 when the caller
// does not send X-Confirm: yes.
func TestGeneralHandler_RunAction_RequiresConfirmHeader(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
		Config: &models.CustomServiceConfig{
			Actions: []models.CustomActionConfig{
				{ID: "restart", Label: "Restart", Path: "/restart", Confirm: true},
			},
		},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	var ranAction bool
	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{
		runAction: func(context.Context, string, string, *models.CustomServiceConfig, string) (general.ActionResult, error) {
			ranAction = true
			return general.ActionResult{Status: http.StatusOK}, nil
		},
	}}
	r := newGeneralTestRouter(t, handler)

	rec := doRequest(t, r, http.MethodPost, "/api/general/general-1/actions/restart", "", nil)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusPreconditionRequired, rec.Body.String())
	}
	if ranAction {
		t.Fatal("expected the action to not run without confirmation")
	}
}

// With the confirm header present, a confirm:true action runs and its
// result is returned as {status, body}.
func TestGeneralHandler_RunAction_OKWithConfirmHeader(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
		Config: &models.CustomServiceConfig{
			Actions: []models.CustomActionConfig{
				{ID: "restart", Label: "Restart", Path: "/restart", Confirm: true},
			},
		},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{
		runAction: func(_ context.Context, rawURL, _ string, _ *models.CustomServiceConfig, actionID string) (general.ActionResult, error) {
			if actionID != "restart" {
				t.Fatalf("actionID = %q, want %q", actionID, "restart")
			}
			if rawURL != "http://localhost:9000" {
				t.Fatalf("rawURL = %q, want %q", rawURL, "http://localhost:9000")
			}
			return general.ActionResult{Status: http.StatusAccepted, Body: `{"ok":true}`}, nil
		},
	}}
	r := newGeneralTestRouter(t, handler)

	rec := doRequest(t, r, http.MethodPost, "/api/general/general-1/actions/restart", "", map[string]string{"X-Confirm": "yes"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != http.StatusAccepted {
		t.Errorf("resp.Status = %d, want %d", resp.Status, http.StatusAccepted)
	}
	if resp.Body != `{"ok":true}` {
		t.Errorf("resp.Body = %q, want %q", resp.Body, `{"ok":true}`)
	}
}

// An action without confirm:true runs immediately, no header required.
func TestGeneralHandler_RunAction_NoConfirmRequired(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
		Config: &models.CustomServiceConfig{
			Actions: []models.CustomActionConfig{
				{ID: "refresh", Label: "Refresh", Path: "/refresh"},
			},
		},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	rec := doRequest(t, r, http.MethodPost, "/api/general/general-1/actions/refresh", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// An unknown action id is a 404, not a 428 or 500.
func TestGeneralHandler_RunAction_UnknownAction(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := db.CreateService(ctx, &models.ServiceConfiguration{
		InstanceID:  "general-1",
		DisplayName: "Custom",
		URL:         "http://localhost:9000",
		Config:      &models.CustomServiceConfig{},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	rec := doRequest(t, r, http.MethodPost, "/api/general/general-1/actions/nope", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// POST /api/general/test never persists anything and reports the stub
// engine's health/stats verbatim.
func TestGeneralHandler_Test_DoesNotPersist(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{
		checkHealth: func(context.Context, string, string, *models.CustomServiceConfig) (models.ServiceHealth, int) {
			return models.ServiceHealth{Status: "online", Version: "1.2.3"}, http.StatusOK
		},
		fetchStats: func(context.Context, string, string, *models.CustomServiceConfig) (map[string]general.StatValue, error) {
			return map[string]general.StatValue{"Queue": {Label: "Queue", Display: "3", Raw: float64(3)}}, nil
		},
	}}
	r := newGeneralTestRouter(t, handler)

	body := `{"url": "http://localhost:9000", "apiKey": "k", "config": {"health": {"path": "/health"}}}`
	rec := doRequest(t, r, http.MethodPost, "/api/general/test", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp generalTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "online" || resp.Version != "1.2.3" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Stats["Queue"].Display != "3" {
		t.Fatalf("unexpected stats: %+v", resp.Stats)
	}

	ctx := context.Background()
	services, err := db.GetAllServices(ctx)
	if err != nil {
		t.Fatalf("GetAllServices: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected /test to persist nothing, found %d services", len(services))
	}
}

// A structurally invalid config in the test request is rejected the same
// way as PUT .../config.
func TestGeneralHandler_Test_ValidationError(t *testing.T) {
	db, cleanup := setupUIPreferencesTestDB(t)
	defer cleanup()

	handler := &GeneralHandler{db: db, engine: &stubGeneralEngine{}}
	r := newGeneralTestRouter(t, handler)

	body := `{"url": "http://localhost:9000", "config": {"health": {"statusPath": "status"}}}`
	rec := doRequest(t, r, http.MethodPost, "/api/general/test", body, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// Contract test: buildGeneralServiceUpdate must carry stats as
// label -> {display, raw, unit} and actions as [{id, label, confirm}],
// nested under Stats["general"] so the health job's Details["general"]
// (legacy scalar fields) and Status are left untouched by this internal
// update - see poller.go's runGeneralHealth vs runGeneralStats split.
func TestBuildGeneralServiceUpdate_PayloadShape(t *testing.T) {
	stats := map[string]general.StatValue{
		"Queue": {Label: "Queue", Display: "3 items", Raw: float64(3), Unit: "items"},
	}
	actions := []models.CustomActionConfig{
		{ID: "restart", Label: "Restart", Confirm: true},
	}

	health := buildGeneralServiceUpdate("general-1", stats, actions)

	if health.ServiceID != "general-1" {
		t.Fatalf("ServiceID = %q, want %q", health.ServiceID, "general-1")
	}

	generalStats, ok := health.Stats["general"].(map[string]any)
	if !ok {
		t.Fatalf("Stats[general] missing or wrong type: %#v", health.Stats)
	}

	statOut, ok := generalStats["stats"].(map[string]any)
	if !ok {
		t.Fatalf("Stats[general][stats] missing or wrong type: %#v", generalStats)
	}
	queue, ok := statOut["Queue"].(map[string]any)
	if !ok {
		t.Fatalf("Stats[general][stats][Queue] missing or wrong type: %#v", statOut)
	}
	if queue["display"] != "3 items" || queue["unit"] != "items" {
		t.Fatalf("unexpected Queue stat shape: %#v", queue)
	}
	if raw, ok := queue["raw"].(float64); !ok || raw != 3 {
		t.Fatalf("unexpected Queue raw value: %#v", queue["raw"])
	}

	actionOut, ok := generalStats["actions"].([]map[string]any)
	if !ok {
		t.Fatalf("Stats[general][actions] missing or wrong type: %#v", generalStats)
	}
	if len(actionOut) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actionOut))
	}
	if actionOut[0]["id"] != "restart" || actionOut[0]["label"] != "Restart" || actionOut[0]["confirm"] != true {
		t.Fatalf("unexpected action shape: %#v", actionOut[0])
	}

	// Details.general must not be touched by this builder - it stays
	// whatever the health job (runGeneralHealth) already populated.
	if health.Details != nil {
		t.Fatalf("expected Details to be nil, got %#v", health.Details)
	}
}
