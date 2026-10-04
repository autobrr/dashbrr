package discovery

import (
	"fmt"

	"github.com/autobrr/dashbrr/internal/models"
)

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
