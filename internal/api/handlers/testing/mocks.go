// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package testing

import (
	"context"
	"github.com/autobrr/dashbrr/internal/types"
)

// MockDB implements database operations for testing
type MockDB struct {
	FindServiceByFunc  func(ctx context.Context, params types.FindServiceParams) (*types.ServiceConfiguration, error)
	GetAllServicesFunc func() ([]types.ServiceConfiguration, error)
	CreateServiceFunc  func(*types.ServiceConfiguration) error
	UpdateServiceFunc  func(*types.ServiceConfiguration) error
	DeleteServiceFunc  func(string) error
}

// FindServiceBy implements the database method
func (m *MockDB) FindServiceBy(ctx context.Context, params types.FindServiceParams) (*types.ServiceConfiguration, error) {
	if m.FindServiceByFunc != nil {
		return m.FindServiceByFunc(ctx, params)
	}
	return nil, nil
}

// GetAllServices implements the database method
func (m *MockDB) GetAllServices() ([]types.ServiceConfiguration, error) {
	if m.GetAllServicesFunc != nil {
		return m.GetAllServicesFunc()
	}
	return []types.ServiceConfiguration{}, nil
}

// CreateService implements the database method
func (m *MockDB) CreateService(config *types.ServiceConfiguration) error {
	if m.CreateServiceFunc != nil {
		return m.CreateServiceFunc(config)
	}
	return nil
}

// UpdateService implements the database method
func (m *MockDB) UpdateService(config *types.ServiceConfiguration) error {
	if m.UpdateServiceFunc != nil {
		return m.UpdateServiceFunc(config)
	}
	return nil
}

// DeleteService implements the database method
func (m *MockDB) DeleteService(instanceID string) error {
	if m.DeleteServiceFunc != nil {
		return m.DeleteServiceFunc(instanceID)
	}
	return nil
}
