package discovery

import (
	"context"
	"fmt"
	"os"
	"strings"

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
			fmt.Printf("Warning: Failed to parse labels for container %s: %v\n", container.ID[:12], err)
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
	serviceType := labels[GetLabelKey(labelTypeKey)]
	if serviceType == "" {
		return nil, fmt.Errorf("service type label not found")
	}

	url := labels[GetLabelKey(labelURLKey)]
	if url == "" {
		return nil, fmt.Errorf("service URL label not found")
	}

	// Handle environment variable substitution in API key
	apiKey := labels[GetLabelKey(labelAPIKeyKey)]
	if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(apiKey, "${"), "}")
		apiKey = os.Getenv(envVar)
		if apiKey == "" {
			return nil, fmt.Errorf("environment variable %s not set for API key", envVar)
		}
	}

	// Get optional display name or use service type
	displayName := labels[GetLabelKey(labelNameKey)]
	if displayName == "" {
		displayName = strings.Title(serviceType)
	}

	// Check if service is explicitly disabled
	if enabled := labels[GetLabelKey(labelEnabledKey)]; enabled == "false" {
		return nil, nil
	}

	// Generate instance ID based on service type
	instanceID := fmt.Sprintf("%s-docker", serviceType)

	return &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: displayName,
		URL:         url,
		APIKey:      apiKey,
	}, nil
}

// Close closes the Docker client connection
func (d *DockerDiscovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
