// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package testing

import (
	"context"
	"github.com/autobrr/dashbrr/internal/domain"
)

// MockDB implements database operations for testing
type MockDB struct {
	FindServiceByFunc  func(ctx context.Context, params domain.FindServiceParams) (*domain.ServiceConfiguration, error)
	GetAllServicesFunc func() ([]domain.ServiceConfiguration, error)
	CreateServiceFunc  func(*domain.ServiceConfiguration) error
	UpdateServiceFunc  func(*domain.ServiceConfiguration) error
	DeleteServiceFunc  func(string) error
}

// FindServiceBy implements the database method
func (m *MockDB) FindServiceBy(ctx context.Context, params domain.FindServiceParams) (*domain.ServiceConfiguration, error) {
	if m.FindServiceByFunc != nil {
		return m.FindServiceByFunc(ctx, params)
	}
	return nil, nil
}

// GetAllServices implements the database method
func (m *MockDB) GetAllServices() ([]domain.ServiceConfiguration, error) {
	if m.GetAllServicesFunc != nil {
		return m.GetAllServicesFunc()
	}
	return []domain.ServiceConfiguration{}, nil
}

// CreateService implements the database method
func (m *MockDB) CreateService(config *domain.ServiceConfiguration) error {
	if m.CreateServiceFunc != nil {
		return m.CreateServiceFunc(config)
	}
	return nil
}

// UpdateService implements the database method
func (m *MockDB) UpdateService(config *domain.ServiceConfiguration) error {
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
