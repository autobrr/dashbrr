package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHealthService(t *testing.T) {
	hs := NewHealthService()
	assert.NotNil(t, hs)
	assert.NotNil(t, hs.monitoredServices)
	assert.NotNil(t, hs.healthChecks)
}

func TestCheckServiceHealth(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		url         string
		apiKey      string
		wantStatus  string
		wantCode    int
	}{
		{
			name:        "Empty URL",
			serviceType: "test",
			url:         "",
			apiKey:      "test-key",
			wantStatus:  "error",
			wantCode:    400,
		},
		{
			name:        "Invalid Service Type",
			serviceType: "invalid-service",
			url:         "http://test.com",
			apiKey:      "test-key",
			wantStatus:  "error",
			wantCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health, code := CheckServiceHealth(tt.serviceType, tt.url, tt.apiKey)
			assert.Equal(t, tt.wantStatus, health.Status)
			assert.Equal(t, tt.wantCode, code)
		})
	}
}

func TestHealthService_StartStopMonitoring(t *testing.T) {
	hs := NewHealthService()
	instanceID := "test-instance"

	// Start monitoring
	hs.StartMonitoring(instanceID, func(ctx context.Context) (*HealthCheck, error) {
		return &HealthCheck{
			Status:      "healthy",
			LastChecked: time.Now(),
		}, nil
	})

	// Wait for the health check to be stored in the map
	var health *HealthCheck
	startTime := time.Now()
	for {
		if time.Since(startTime) > time.Second {
			t.Fatal("Timeout waiting for health check to be stored")
		}
		health = hs.GetHealth(instanceID)
		if health != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, "healthy", health.Status)

	// Stop monitoring
	hs.StopMonitoring(instanceID)

	// Verify monitoring is stopped
	hs.mu.RLock()
	_, exists := hs.monitoredServices[instanceID]
	hs.mu.RUnlock()
	assert.False(t, exists, "Service should be removed after stopping")
}

func TestHealthService_GetAllHealth(t *testing.T) {
	hs := NewHealthService()

	// Add some test health checks
	hs.mu.Lock()
	hs.healthChecks["test1"] = &HealthCheck{Status: "healthy"}
	hs.healthChecks["test2"] = &HealthCheck{Status: "error"}
	hs.mu.Unlock()

	// Get all health checks
	allHealth := hs.GetAllHealth()

	// Verify we got the expected number of health checks
	assert.Len(t, allHealth, 2)

	// Verify each health check exists and has the expected status
	test1Health, exists := allHealth["test1"]
	assert.True(t, exists, "test1 health check should exist")
	if test1Health != nil {
		assert.Equal(t, "healthy", test1Health.Status)
	} else {
		t.Error("test1 health check should not be nil")
	}

	test2Health, exists := allHealth["test2"]
	assert.True(t, exists, "test2 health check should exist")
	if test2Health != nil {
		assert.Equal(t, "error", test2Health.Status)
	} else {
		t.Error("test2 health check should not be nil")
	}
}

func TestHealthService_Cleanup(t *testing.T) {
	hs := NewHealthService()
	instanceID := "test-instance"

	// Start monitoring
	hs.StartMonitoring(instanceID, func(ctx context.Context) (*HealthCheck, error) {
		return &HealthCheck{
			Status:      "healthy",
			LastChecked: time.Now(),
		}, nil
	})

	// Wait for the health check to be stored
	var health *HealthCheck
	startTime := time.Now()
	for {
		if time.Since(startTime) > time.Second {
			t.Fatal("Timeout waiting for health check to be stored")
		}
		health = hs.GetHealth(instanceID)
		if health != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cleanup
	hs.Cleanup()

	// Verify everything is cleaned up
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	assert.Empty(t, hs.monitoredServices)
	assert.Empty(t, hs.healthChecks)
}

func TestHealthService_ConcurrentAccess(t *testing.T) {
	hs := NewHealthService()
	instanceID := "test-instance"
	done := make(chan bool)
	const numGoroutines = 10

	// Start multiple goroutines accessing the service
	for i := 0; i < numGoroutines; i++ {
		go func() {
			mockCheckFn := func(ctx context.Context) (*HealthCheck, error) {
				return &HealthCheck{
					Status:      "healthy",
					LastChecked: time.Now(),
				}, nil
			}

			hs.StartMonitoring(instanceID, mockCheckFn)

			// Wait for health check to be stored
			startTime := time.Now()
			for {
				if time.Since(startTime) > time.Second {
					break
				}
				if hs.GetHealth(instanceID) != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			_ = hs.GetAllHealth()
			hs.StopMonitoring(instanceID)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Goroutine did not complete within timeout")
		}
	}
}
