package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/discovery"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/spf13/cobra"
)

func ConfigCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "config",
		Long:  `config`,
		Example: `  dashbrr config 
  dashbrr config --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ConfigImportCommand())
	command.AddCommand(ConfigExportCommand())
	command.AddCommand(ConfigDiscoverCommand())

	return command
}

func ConfigImportCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "import",
		Short: "import",
		Long:  `import`,
		Example: `  dashbrr config import
  dashbrr config import --help`,
		//SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		services, err := discovery.ImportConfig(filePath)
		if err != nil {
			return fmt.Errorf("failed to import config: %v", err)
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		if err := handleDiscoveredServices(cmd.Context(), db, services); err != nil {
			return err
		}

		return nil
	}

	return command
}

func ConfigExportCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "export",
		Short: "export",
		Long:  `export`,
		Example: `  dashbrr config export
  dashbrr config export --help`,
		//SilenceUsage: true,
	}

	var (
		format      = ""
		outputPath  = ""
		maskSecrets = false
	)

	command.Flags().StringVarP(&format, "format", "f", "yaml", "Output format (yaml or json)")
	command.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")
	command.Flags().BoolVarP(&maskSecrets, "mask-secrets", "m", false, "Mask API keys")

	command.RunE = func(cmd *cobra.Command, args []string) error {
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

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// Get all services from database
		services, err := db.GetAllServices(context.Background())
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

	return command
}

func ConfigDiscoverCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "discover",
		Short: "discover",
		Long:  `discover`,
		Example: `  dashbrr config discover
  dashbrr config discover --help`,
	}

	var (
		useDocker = false
		useK8s    = false
	)

	command.Flags().BoolVarP(&useDocker, "docker", "d", false, "Use Docker discovery")
	command.Flags().BoolVarP(&useK8s, "k8s", "k", false, "Use Kubernetes discovery")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		// If no specific platform is selected, try both
		if !useDocker && !useK8s {
			useDocker = true
			useK8s = true
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// Create discovery manager
		manager, err := discovery.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize service discovery: %v", err)
		}
		defer manager.Close()

		// Discover services
		services, err := manager.DiscoverAll(cmd.Context())
		if err != nil {
			return fmt.Errorf("service discovery failed: %v", err)
		}

		if err := handleDiscoveredServices(cmd.Context(), db, services); err != nil {
			return err
		}

		return nil
	}

	return command
}

// handleDiscoveredServices processes discovered services
func handleDiscoveredServices(ctx context.Context, db *database.DB, services []models.ServiceConfiguration) error {
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
		existing, err := db.FindServiceBy(ctx, types.FindServiceParams{URL: service.URL})
		if err != nil {
			fmt.Printf("Warning: Failed to check for existing service %s: %v\n", service.URL, err)
			continue
		}
		if existing != nil {
			fmt.Printf("Skipping %s: Service already exists\n", service.URL)
			continue
		}

		// Add new service
		if err := db.CreateService(ctx, &service); err != nil {
			fmt.Printf("Warning: Failed to add service %s: %v\n", service.URL, err)
			continue
		}
		fmt.Printf("Added service: %s (%s)\n", service.DisplayName, service.URL)
	}

	return nil
}
