package general

import (
	"context"
	"fmt"
	"github.com/autobrr/dashbrr/internal/types"
	"net/url"
	"strconv"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
)

// AddCommand handles adding a new General service
type AddCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewAddCommand(db *database.DB) *AddCommand {
	return &AddCommand{
		BaseCommand: base.NewBaseCommand(
			"service general add",
			"Add a General service configuration",
			"<url> [name] [api-key]\n\n"+
				"Example:\n"+
				"  dashbrr run service general add http://my.general.service/healthz/liveness MyService\n"+
				"  dashbrr run service general add http://my.general.service/healthz/liveness MyService optional-api-key",
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
	prefix := "general-"

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
	if len(args) < 1 || len(args) > 3 {
		return fmt.Errorf("incorrect number of arguments\n\n%s", c.Usage())
	}

	serviceURL := args[0]
	displayName := "General" // Default name
	apiKey := ""             // Default empty string

	// Handle optional arguments
	switch len(args) {
	case 3:
		apiKey = args[2]
		fallthrough
	case 2:
		displayName = args[1]
	}

	// Validate URL
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Check if service already exists
	existing, err := c.db.FindServiceBy(context.Background(), types.FindServiceParams{URL: serviceURL})
	if err != nil {
		return fmt.Errorf("failed to check for existing service: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("service with URL %s already exists", serviceURL)
	}

	// Create General service
	generalService := models.NewGeneralService()

	// Perform health check to validate connection
	health, _ := generalService.CheckHealth(serviceURL, apiKey)

	if health.Status != "online" {
		return fmt.Errorf("failed to connect to General service: %s", health.Message)
	}

	// Get next available instance ID
	instanceID, err := c.getNextInstanceID()
	if err != nil {
		return fmt.Errorf("failed to generate instance ID: %v", err)
	}

	// Create service configuration
	service := &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: displayName,
		URL:         serviceURL,
		APIKey:      apiKey,
	}

	if err := c.db.CreateService(service); err != nil {
		return fmt.Errorf("failed to save service configuration: %v", err)
	}

	fmt.Printf("General service added successfully:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Printf("  Version: %s\n", health.Version)
	fmt.Printf("  Status: %s\n", health.Status)
	fmt.Printf("  Instance ID: %s\n", instanceID)

	return nil
}

// RemoveCommand handles removing an General service
type RemoveCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewRemoveCommand(db *database.DB) *RemoveCommand {
	return &RemoveCommand{
		BaseCommand: base.NewBaseCommand(
			"service general remove",
			"Remove an General service configuration",
			"<url>\n\n"+
				"Example:\n"+
				"  dashbrr run service general remove http://localhost:7475",
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
	service, err := c.db.FindServiceBy(context.Background(), types.FindServiceParams{URL: serviceURL})
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

	fmt.Printf("General service removed successfully:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Printf("  Instance ID: %s\n", service.InstanceID)

	return nil
}

// ListCommand handles listing General services
type ListCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewListCommand(db *database.DB) *ListCommand {
	return &ListCommand{
		BaseCommand: base.NewBaseCommand(
			"service general list",
			"List configured General services",
			"",
		),
		db: db,
	}
}

func (c *ListCommand) Execute(ctx context.Context, args []string) error {
	// Get all configured services
	services, err := c.db.GetAllServices()
	if err != nil {
		return fmt.Errorf("failed to retrieve services: %v", err)
	}

	if len(services) == 0 {
		fmt.Println("No General services configured.")
		return nil
	}

	fmt.Println("Configured General Services:")
	for _, service := range services {
		// Only show general services
		if strings.HasPrefix(service.InstanceID, "general-") {
			fmt.Printf("  - URL: %s\n", service.URL)
			fmt.Printf("    Instance ID: %s\n", service.InstanceID)

			// Try to get health info which includes version
			generalService := models.NewGeneralService()
			if health, _ := generalService.CheckHealth(service.URL, service.APIKey); health.Status == "online" {
				fmt.Printf("    Version: %s\n", health.Version)
				fmt.Printf("    Status: %s\n", health.Status)
			}
		}
	}

	return nil
}
