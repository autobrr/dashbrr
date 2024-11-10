package discovery

import (
	"context"
	"fmt"

	"github.com/autobrr/dashbrr/internal/models"
)

// ServiceDiscoverer defines the interface for service discovery implementations
type ServiceDiscoverer interface {
	// DiscoverServices finds and returns service configurations
	DiscoverServices(ctx context.Context) ([]models.ServiceConfiguration, error)
	// Close cleans up any resources used by the discoverer
	Close() error
}

// Manager handles multiple service discovery methods
type Manager struct {
	discoverers []ServiceDiscoverer
}

// NewManager creates a new discovery manager
func NewManager() (*Manager, error) {
	var discoverers []ServiceDiscoverer

	// Try to initialize Docker discovery
	if docker, err := NewDockerDiscovery(); err == nil {
		discoverers = append(discoverers, docker)
	}

	// Try to initialize Kubernetes discovery
	if k8s, err := NewKubernetesDiscovery(); err == nil {
		discoverers = append(discoverers, k8s)
	}

	if len(discoverers) == 0 {
		return nil, fmt.Errorf("no service discovery methods available")
	}

	return &Manager{
		discoverers: discoverers,
	}, nil
}

// DiscoverAll finds services using all available discovery methods
func (m *Manager) DiscoverAll(ctx context.Context) ([]models.ServiceConfiguration, error) {
	var allServices []models.ServiceConfiguration

	for _, discoverer := range m.discoverers {
		services, err := discoverer.DiscoverServices(ctx)
		if err != nil {
			// Log error but continue with other discoverers
			fmt.Printf("Warning: Service discovery error: %v\n", err)
			continue
		}
		allServices = append(allServices, services...)
	}

	return allServices, nil
}

// Close cleans up all discoverers
func (m *Manager) Close() error {
	var lastErr error
	for _, discoverer := range m.discoverers {
		if err := discoverer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ValidateService checks if a discovered service configuration is valid
func ValidateService(service models.ServiceConfiguration) error {
	if service.InstanceID == "" {
		return fmt.Errorf("instance ID is required")
	}
	if service.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}
	if service.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if service.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}
