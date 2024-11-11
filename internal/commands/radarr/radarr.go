package radarr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/radarr"
)

// AddCommand handles adding a new radarr service
type AddCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewAddCommand(db *database.DB) *AddCommand {
	return &AddCommand{
		BaseCommand: base.NewBaseCommand(
			"service radarr add",
			"Add an radarr service configuration",
			"<url> <api-key>\n\n"+
				"Example:\n"+
				"  dashbrr run service radarr add http://localhost:7878 your-api-key",
		),
		db: db,
	}
}

func (c *AddCommand) getNextInstanceID() (string, error) {
	services, err := c.db.GetAllServices()
	if err != nil {
		return "", fmt.Errorf("failed to get services: %v", err)
	}

	maxNum := 0
	prefix := "radarr-"

	for _, service := range services {
		if strings.HasPrefix(service.InstanceID, prefix) {
			numStr := strings.TrimPrefix(service.InstanceID, prefix)
			if num, err := strconv.Atoi(numStr); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("%s%d", prefix, maxNum+1), nil
}

func (c *AddCommand) Execute(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("insufficient arguments\n\n%s", c.Usage())
	}

	serviceURL := args[0]
	apiKey := args[1]

	// Validate URL
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Check if service already exists
	existing, err := c.db.GetServiceByURL(serviceURL)
	if err != nil {
		return fmt.Errorf("failed to check for existing service: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("service with URL %s already exists", serviceURL)
	}

	// Create radarr service
	radarrService := radarr.NewRadarrService()

	// Perform health check to validate connection
	health, _ := radarrService.CheckHealth(serviceURL, apiKey)

	if health.Status == "error" || health.Status == "offline" {
		return fmt.Errorf("failed to connect to radarr service: %s", health.Message)
	}

	// Get next available instance ID
	instanceID, err := c.getNextInstanceID()
	if err != nil {
		return fmt.Errorf("failed to generate instance ID: %v", err)
	}

	// Create service configuration
	service := &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: "radarr",
		URL:         serviceURL,
		APIKey:      apiKey,
	}

	if err := c.db.CreateService(service); err != nil {
		return fmt.Errorf("failed to save service configuration: %v", err)
	}

	fmt.Printf("radarr service added successfully:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Printf("  Version: %s\n", health.Version)
	fmt.Printf("  Status: %s\n", health.Status)
	fmt.Printf("  Instance ID: %s\n", instanceID)

	return nil
}

// RemoveCommand handles removing an radarr service
type RemoveCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewRemoveCommand(db *database.DB) *RemoveCommand {
	return &RemoveCommand{
		BaseCommand: base.NewBaseCommand(
			"service radarr remove",
			"Remove an radarr service configuration",
			"<url>\n\n"+
				"Example:\n"+
				"  dashbrr run service radarr remove http://localhost:7878",
		),
		db: db,
	}
}

func (c *RemoveCommand) Execute(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("insufficient arguments\n\n%s", c.Usage())
	}

	serviceURL := args[0]

	// Find service by URL
	service, err := c.db.GetServiceByURL(serviceURL)
	if err != nil {
		return fmt.Errorf("failed to find service: %v", err)
	}
	if service == nil {
		return fmt.Errorf("no service found with URL: %s", serviceURL)
	}

	// Delete service
	if err := c.db.DeleteService(service.InstanceID); err != nil {
		return fmt.Errorf("failed to remove service: %v", err)
	}

	fmt.Printf("radarr service removed successfully:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Printf("  Instance ID: %s\n", service.InstanceID)

	return nil
}

// ListCommand handles listing radarr services
type ListCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewListCommand(db *database.DB) *ListCommand {
	return &ListCommand{
		BaseCommand: base.NewBaseCommand(
			"service radarr list",
			"List configured radarr services",
			"",
		),
		db: db,
	}
}

func (c *ListCommand) Execute(ctx context.Context, args []string) error {
	services, err := c.db.GetAllServices()
	if err != nil {
		return fmt.Errorf("failed to retrieve services: %v", err)
	}

	if len(services) == 0 {
		fmt.Println("No radarr services configured.")
		return nil
	}

	fmt.Println("Configured radarr Services:")
	for _, service := range services {
		if strings.HasPrefix(service.InstanceID, "radarr-") {
			fmt.Printf("  - URL: %s\n", service.URL)
			fmt.Printf("    Instance ID: %s\n", service.InstanceID)

			// Try to get health info which includes version
			radarrService := radarr.NewRadarrService()
			if health, _ := radarrService.CheckHealth(service.URL, service.APIKey); health.Status != "" {
				if health.Version != "" {
					fmt.Printf("    Version: %s\n", health.Version)
				}
				fmt.Printf("    Status: %s\n", health.Status)
			}
		}
	}

	return nil
}
