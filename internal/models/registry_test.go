// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"testing"
)

func TestNewServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry()
	if registry == nil {
		t.Error("Expected non-nil registry")
	}
}

func TestCreateService(t *testing.T) {
	registry := NewServiceRegistry()

	// Test unknown service type
	service := registry.CreateService("nonexistent")
	if service != nil {
		t.Error("Expected nil for unknown service type")
	}

	// Test case insensitivity
	// Mock a service creator for testing
	originalAutobrrService := NewAutobrrService
	originalQuiService := NewQuiService
	originalLidarrService := NewLidarrService
	defer func() {
		NewAutobrrService = originalAutobrrService
		NewQuiService = originalQuiService
		NewLidarrService = originalLidarrService
	}()

	called := false
	NewAutobrrService = func() ServiceHealthChecker {
		called = true
		return nil
	}

	// Test with different cases
	registry.CreateService("AUTOBRR")
	if !called {
		t.Error("Service creator not called for uppercase service type")
	}

	called = false
	registry.CreateService("autobrr")
	if !called {
		t.Error("Service creator not called for lowercase service type")
	}

	called = false
	NewQuiService = func() ServiceHealthChecker {
		called = true
		return nil
	}
	registry.CreateService("qui")
	if !called {
		t.Error("Service creator not called for qui service type")
	}

	called = false
	NewLidarrService = func() ServiceHealthChecker {
		called = true
		return nil
	}
	registry.CreateService("lidarr")
	if !called {
		t.Error("Service creator not called for lidarr service type")
	}
}
