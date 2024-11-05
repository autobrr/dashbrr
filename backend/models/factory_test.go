package models

import (
	"testing"
)

func TestNewServiceFactory(t *testing.T) {
	factory := NewServiceFactory()
	if factory == nil {
		t.Error("Expected non-nil factory")
	}
}

func TestCreateService(t *testing.T) {
	factory := NewServiceFactory()

	// Test unknown service type
	service := factory.CreateService("nonexistent")
	if service != nil {
		t.Error("Expected nil for unknown service type")
	}

	// Test case insensitivity
	// Mock a service creator for testing
	originalAutobrrService := NewAutobrrService
	defer func() { NewAutobrrService = originalAutobrrService }()

	called := false
	NewAutobrrService = func() ServiceHealthChecker {
		called = true
		return nil
	}

	// Test with different cases
	factory.CreateService("AUTOBRR")
	if !called {
		t.Error("Service creator not called for uppercase service type")
	}

	called = false
	factory.CreateService("autobrr")
	if !called {
		t.Error("Service creator not called for lowercase service type")
	}
}
