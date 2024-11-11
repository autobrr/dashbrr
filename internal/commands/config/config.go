package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/discovery"
)

// ConfigCommand handles service discovery and configuration
type ConfigCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewConfigCommand(db *database.DB) *ConfigCommand {
	return &ConfigCommand{
		BaseCommand: base.NewBaseCommand(
			"config",
			"Service discovery and configuration management",
			"Usage:\n"+
				"  dashbrr run config discover [--docker] [--k8s]\n"+
				"  dashbrr run config import <file>\n"+
				"  dashbrr run config export [--format=<yaml|json>] [--mask-secrets] [--output=<file>]\n\n"+
				"Examples:\n"+
				"  # Discover services from Docker labels\n"+
				"  dashbrr run config discover --docker\n\n"+
				"  # Import services from external config\n"+
				"  dashbrr run config import services.yaml\n\n"+
				"  # Export configurations\n"+
				"  dashbrr run config export --format=yaml --mask-secrets --output=services.yaml",
		),
		db: db,
	}
}

func (c *ConfigCommand) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no subcommand specified\n\n%s", c.Usage())
	}

	subcommand := args[0]
	subcommandArgs := args[1:]

	switch subcommand {
	case "discover":
		return c.handleDiscover(ctx, subcommandArgs)
	case "import":
		return c.handleImport(ctx, subcommandArgs)
	case "export":
		return c.handleExport(ctx, subcommandArgs)
	default:
		return fmt.Errorf("unknown subcommand: %s\n\n%s", subcommand, c.Usage())
	}
}

// handleDiscover discovers services from Docker/Kubernetes labels
func (c *ConfigCommand) handleDiscover(ctx context.Context, args []string) error {
	// Parse flags
	useDocker := false
	useK8s := false
	for _, arg := range args {
		switch arg {
		case "--docker":
			useDocker = true
		case "--k8s":
			useK8s = true
		default:
			return fmt.Errorf("unknown flag: %s\n\n%s", arg, c.Usage())
		}
	}

	// If no specific platform is selected, try both
	if !useDocker && !useK8s {
		useDocker = true
		useK8s = true
	}

	// Create discovery manager
	manager, err := discovery.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize service discovery: %v", err)
	}
	defer manager.Close()

	// Discover services
	services, err := manager.DiscoverAll(ctx)
	if err != nil {
		return fmt.Errorf("service discovery failed: %v", err)
	}

	return c.handleDiscoveredServices(services)
}

// handleImport imports services from an external config file
func (c *ConfigCommand) handleImport(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("config import requires a file path\n\n%s", c.Usage())
	}

	filePath := args[0]
	services, err := discovery.ImportConfig(filePath)
	if err != nil {
		return fmt.Errorf("failed to import config: %v", err)
	}

	return c.handleDiscoveredServices(services)
}

// handleExport exports the current configuration
func (c *ConfigCommand) handleExport(ctx context.Context, args []string) error {
	var format, outputPath string
	maskSecrets := false

	// Parse flags
	for _, arg := range args {
		if strings.HasPrefix(arg, "--format=") {
			format = strings.TrimPrefix(arg, "--format=")
		} else if strings.HasPrefix(arg, "--output=") {
			outputPath = strings.TrimPrefix(arg, "--output=")
		} else if arg == "--mask-secrets" {
			maskSecrets = true
		} else {
			return fmt.Errorf("unknown flag: %s\n\n%s", arg, c.Usage())
		}
	}

	// Set default format and output path if not specified
	if format == "" {
		format = "yaml"
	}
	if outputPath == "" {
		outputPath = fmt.Sprintf("dashbrr-services.%s", format)
	}

	// Validate format
	switch format {
	case "yaml", "yml", "json":
		// Ensure output path has correct extension
		if filepath.Ext(outputPath) == "" {
			outputPath += "." + format
		}
	default:
		return fmt.Errorf("unsupported format: %s (use yaml or json)", format)
	}

	// Get all services from database
	services, err := c.db.GetAllServices()
	if err != nil {
		return fmt.Errorf("failed to retrieve services: %v", err)
	}

	// Export configuration
	if err := discovery.ExportConfig(services, outputPath, maskSecrets); err != nil {
		return fmt.Errorf("failed to export config: %v", err)
	}

	fmt.Printf("Configuration exported to %s\n", outputPath)
	if maskSecrets {
		fmt.Println("API keys have been masked. Use environment variables to provide the actual keys.")
	}

	return nil
}

// handleDiscoveredServices processes discovered services
func (c *ConfigCommand) handleDiscoveredServices(services []models.ServiceConfiguration) error {
	if len(services) == 0 {
		fmt.Println("No services discovered.")
		return nil
	}

	// Group services by type for display
	servicesByType := make(map[string][]string)
	for _, service := range services {
		serviceType := strings.Split(service.InstanceID, "-")[0]
		info := fmt.Sprintf("  - %s (URL: %s)", service.DisplayName, service.URL)
		servicesByType[serviceType] = append(servicesByType[serviceType], info)
	}

	// Display discovered services
	fmt.Printf("Discovered %d services:\n\n", len(services))
	for serviceType, infos := range servicesByType {
		fmt.Printf("%s:\n", strings.Title(serviceType))
		for _, info := range infos {
			fmt.Println(info)
		}
		fmt.Println()
	}

	// Ask for confirmation before adding services
	fmt.Print("Would you like to add these services? [y/N] ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Add services to database
	for _, service := range services {
		// Check if service already exists
		existing, err := c.db.GetServiceByURL(service.URL)
		if err != nil {
			fmt.Printf("Warning: Failed to check for existing service %s: %v\n", service.URL, err)
			continue
		}
		if existing != nil {
			fmt.Printf("Skipping %s: Service already exists\n", service.URL)
			continue
		}

		// Add new service
		if err := c.db.CreateService(&service); err != nil {
			fmt.Printf("Warning: Failed to add service %s: %v\n", service.URL, err)
			continue
		}
		fmt.Printf("Added service: %s (%s)\n", service.DisplayName, service.URL)
	}

	return nil
}
