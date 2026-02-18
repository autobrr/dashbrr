package discovery

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/autobrr/dashbrr/internal/models"
)

// DockerDiscovery handles service discovery from Docker labels
type DockerDiscovery struct {
	client *client.Client
}

// NewDockerDiscovery creates a new Docker discovery instance
func NewDockerDiscovery() (*DockerDiscovery, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerDiscovery{
		client: cli,
	}, nil
}

// DiscoverServices finds services configured via Docker labels
func (d *DockerDiscovery) DiscoverServices(ctx context.Context) ([]models.ServiceConfiguration, error) {
	// Create a filter for dashbrr service labels
	f := filters.NewArgs()
	f.Add("label", GetLabelKey(labelTypeKey))

	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All:     false,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var services []models.ServiceConfiguration

	for _, container := range containers {
		service, err := d.parseContainerLabels(container.Labels)
		if err != nil {
			log.Warn().Err(err).Str("container", container.ID[:12]).Msg("Failed to parse discovery labels")
			continue
		}
		if service != nil {
			services = append(services, *service)
		}
	}

	return services, nil
}

// parseContainerLabels extracts service configuration from container labels
func (d *DockerDiscovery) parseContainerLabels(labels map[string]string) (*models.ServiceConfiguration, error) {
	parsed, err := parseDiscoveryLabels(labels)
	if err != nil {
		return nil, err
	}
	if !parsed.enabled {
		return nil, nil
	}

	// Generate instance ID based on service type
	instanceID := fmt.Sprintf("%s-docker", parsed.serviceType)

	return &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: parsed.displayName,
		URL:         parsed.url,
		APIKey:      parsed.apiKey,
	}, nil
}

// Close closes the Docker client connection
func (d *DockerDiscovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
