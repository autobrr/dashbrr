package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/autobrr/dashbrr/internal/models"
)

// ConfigFile represents the structure of the external configuration file
type ConfigFile struct {
	Services map[string][]ServiceConfig `json:"services" yaml:"services"`
}

// ServiceConfig represents a service configuration in the external file
type ServiceConfig struct {
	URL         string            `json:"url" yaml:"url"`
	APIKey      string            `json:"apikey" yaml:"apikey"`
	DisplayName string            `json:"name,omitempty" yaml:"name,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// ImportConfig imports service configurations from a file
func ImportConfig(path string) ([]models.ServiceConfiguration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ConfigFile

	// Determine file format from extension
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", filepath.Ext(path))
	}

	var services []models.ServiceConfiguration

	// Convert config file services to service configurations
	for serviceType, configs := range config.Services {
		for i, cfg := range configs {
			// Handle environment variable substitution in API key
			apiKey := cfg.APIKey
			if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
				envVar := strings.TrimSuffix(strings.TrimPrefix(apiKey, "${"), "}")
				apiKey = os.Getenv(envVar)
				if apiKey == "" {
					return nil, fmt.Errorf("environment variable %s not set for API key", envVar)
				}
			}

			// Generate instance ID
			instanceID := fmt.Sprintf("%s-config-%d", serviceType, i+1)

			// Use provided display name or generate from service type
			displayName := cfg.DisplayName
			if displayName == "" {
				displayName = strings.Title(serviceType)
			}

			services = append(services, models.ServiceConfiguration{
				InstanceID:  instanceID,
				DisplayName: displayName,
				URL:         cfg.URL,
				APIKey:      apiKey,
			})
		}
	}

	return services, nil
}

// ExportConfig exports service configurations to a file
func ExportConfig(services []models.ServiceConfiguration, path string, maskSecrets bool) error {
	// Group services by type
	servicesByType := make(map[string][]ServiceConfig)
	for _, service := range services {
		// Extract service type from instance ID
		parts := strings.Split(service.InstanceID, "-")
		if len(parts) == 0 {
			continue
		}
		serviceType := parts[0]

		// Create service config
		config := ServiceConfig{
			URL:         service.URL,
			DisplayName: service.DisplayName,
		}

		// Handle API key masking
		if maskSecrets {
			config.APIKey = "${DASHBRR_" + strings.ToUpper(serviceType) + "_API_KEY}"
		} else {
			config.APIKey = service.APIKey
		}

		servicesByType[serviceType] = append(servicesByType[serviceType], config)
	}

	configFile := ConfigFile{
		Services: servicesByType,
	}

	var data []byte
	var err error

	// Determine file format from extension
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(configFile)
		if err != nil {
			return fmt.Errorf("failed to generate YAML: %w", err)
		}
	case ".json":
		data, err = json.MarshalIndent(configFile, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s", filepath.Ext(path))
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
