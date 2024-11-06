// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package testing

import (
	"github.com/autobrr/dashbrr/backend/models"
)

// MockDB implements database operations for testing
type MockDB struct {
	GetServiceByInstanceIDFunc func(string) (*models.ServiceConfiguration, error)
	GetAllServicesFunc         func() ([]models.ServiceConfiguration, error)
	CreateServiceFunc          func(*models.ServiceConfiguration) error
	UpdateServiceFunc          func(*models.ServiceConfiguration) error
	DeleteServiceFunc          func(string) error
}

// GetServiceByInstanceID implements the database method
func (m *MockDB) GetServiceByInstanceID(id string) (*models.ServiceConfiguration, error) {
	if m.GetServiceByInstanceIDFunc != nil {
		return m.GetServiceByInstanceIDFunc(id)
	}
	return nil, nil
}

// GetAllServices implements the database method
func (m *MockDB) GetAllServices() ([]models.ServiceConfiguration, error) {
	if m.GetAllServicesFunc != nil {
		return m.GetAllServicesFunc()
	}
	return []models.ServiceConfiguration{}, nil
}

// CreateService implements the database method
func (m *MockDB) CreateService(config *models.ServiceConfiguration) error {
	if m.CreateServiceFunc != nil {
		return m.CreateServiceFunc(config)
	}
	return nil
}

// UpdateService implements the database method
func (m *MockDB) UpdateService(config *models.ServiceConfiguration) error {
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
