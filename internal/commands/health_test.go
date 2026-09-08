package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/autobrr/dashbrr/internal/models"
)

// stubHealthChecker is a models.ServiceHealthChecker whose concrete type is
// never *general.GeneralService, so it proves checkServiceHealth's
// config-aware branch is scoped to serviceType == "general" only.
type stubHealthChecker struct {
	called bool
	health models.ServiceHealth
}

func (s *stubHealthChecker) CheckHealth(_ context.Context, _, _ string) (models.ServiceHealth, int) {
	s.called = true
	return s.health, http.StatusOK
}

// Regression test for the `dashbrr health` false-negative: a general service
// instance with a stored CustomServiceConfig whose health check only lives
// at a non-root path, behind a header credential the registry's 3-arg
// checker can't carry, must still be checked with that config - not the
// nil-config legacy probe (GET base URL, apiKey as Bearer), which would 401
// against this fixture the same way the reported bug did.
func TestCheckServiceHealth_GeneralWithConfig_UsesConfigAwareEngine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" && r.Header.Get("X-Api-Key") == "secret-key" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	registry := models.NewServiceRegistry()
	checker := registry.CreateService("general")
	if checker == nil {
		t.Fatal("registry.CreateService(\"general\") returned nil")
	}

	service := models.ServiceConfiguration{
		InstanceID: "general-1",
		URL:        server.URL,
		APIKey:     "secret-key",
		Config: &models.CustomServiceConfig{
			Auth: &models.CustomAuthConfig{Mode: "header", HeaderName: "X-Api-Key"},
			Health: &models.CustomHealthConfig{
				Path:       "/health",
				StatusPath: "status",
				OKValues:   []string{"ok"},
			},
		},
	}

	health, _ := checkServiceHealth(context.Background(), checker, "general", service)
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q (health check must hit the configured path/header, not GET / with a bearer token)", health.Status, "online")
	}
}

// A general service instance with no stored config must keep behaving
// exactly like the pre-D3 legacy probe: GET the base URL, apiKey as Bearer.
func TestCheckServiceHealth_GeneralLegacyNoConfig_StillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	registry := models.NewServiceRegistry()
	checker := registry.CreateService("general")
	if checker == nil {
		t.Fatal("registry.CreateService(\"general\") returned nil")
	}

	service := models.ServiceConfiguration{
		InstanceID: "general-1",
		URL:        server.URL,
	}

	health, _ := checkServiceHealth(context.Background(), checker, "general", service)
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q", health.Status, "online")
	}
}

// Non-general service types must be entirely unaffected by this change,
// even if service.Config happens to be non-nil.
func TestCheckServiceHealth_NonGeneralTypeUnchanged(t *testing.T) {
	stub := &stubHealthChecker{health: models.ServiceHealth{Status: "online"}}
	service := models.ServiceConfiguration{
		InstanceID: "bazarr-1",
		URL:        "http://example.invalid",
		Config:     &models.CustomServiceConfig{},
	}

	health, _ := checkServiceHealth(context.Background(), stub, "bazarr", service)

	if !stub.called {
		t.Fatal("expected the registry checker's 2-arg CheckHealth to be called for a non-general service type")
	}
	if health.Status != "online" {
		t.Fatalf("Status = %q, want %q", health.Status, "online")
	}
}
