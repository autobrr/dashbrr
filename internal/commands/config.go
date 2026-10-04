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
		Short: "Manage config. Import, export, discover",
		Long:  `Manage config. Import, export, discover`,
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
		Use:   "import [file]",
		Short: "import",
		Long:  `import`,
		Example: `  dashbrr config import services.yaml
  dashbrr config import services.json
  dashbrr config import --help`,
		Args: cobra.ExactArgs(1),
		//SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		services, err := discovery.ImportConfig(filePath)
		if err != nil {
			return fmt.Errorf("failed to import config: %v", err)
		}

		db, err := initializeDatabase(cmd)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		if err := handleDiscoveredServices(cmd.Context(), db, services, false); err != nil {
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

		db, err := initializeDatabase(cmd)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// Get all services from database
		services, err := db.GetAllServices(cmd.Context())
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
		assumeYes = false
	)

	command.Flags().BoolVarP(&useDocker, "docker", "d", false, "Use Docker discovery")
	command.Flags().BoolVarP(&useK8s, "k8s", "k", false, "Use Kubernetes discovery")
	command.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Automatically confirm and add discovered services")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		// If no specific platform is selected, try both
		if !useDocker && !useK8s {
			useDocker = true
			useK8s = true
		}

		cfg, _, err := ConfigFromFlags(cmd)
		if err != nil {
			return err
		}

		db, err := initializeDatabase(cmd)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		ran := false

		if useDocker {
			dockerDiscovery, dockerErr := discovery.NewDockerDiscovery()
			if dockerErr != nil {
				fmt.Printf("Warning: Docker discovery unavailable: %v\n", dockerErr)
			} else {
				defer dockerDiscovery.Close()
				services, discoverErr := dockerDiscovery.DiscoverServices(cmd.Context())
				if discoverErr != nil {
					return fmt.Errorf("service discovery failed: %w", discoverErr)
				}
				if err := handleDiscoveredServices(cmd.Context(), db, services, assumeYes); err != nil {
					return err
				}
				ran = true
			}
		}

		if useK8s {
			k8sDiscovery, k8sErr := discovery.NewKubernetesDiscovery(cfg.Discovery.Kubernetes.Namespaces)
			if k8sErr != nil {
				fmt.Printf("Warning: Kubernetes discovery unavailable: %v\n", k8sErr)
			} else {
				if err := syncKubernetesServices(cmd.Context(), db, k8sDiscovery, assumeYes); err != nil {
					return err
				}
				ran = true
			}
		}

		if !ran {
			return fmt.Errorf("failed to initialize any requested discovery backends")
		}

		return nil
	}

	return command
}

// handleDiscoveredServices processes discovered services
func handleDiscoveredServices(ctx context.Context, db *database.DB, services []models.ServiceConfiguration, assumeYes bool) error {
	if len(services) == 0 {
		fmt.Println("No services discovered.")
		return nil
	}

	// Group services by type for display
	servicesByType := make(map[string][]string)
	for _, service := range services {
		serviceType, ok := models.ServiceTypeFromInstanceID(service.InstanceID)
		if !ok {
			serviceType = "unknown"
		}
		info := fmt.Sprintf("  - %s (URL: %s)", service.DisplayName, service.URL)
		servicesByType[serviceType] = append(servicesByType[serviceType], info)
	}

	// Display discovered services
	fmt.Printf("Discovered %d services:\n\n", len(services))
	for serviceType, infos := range servicesByType {
		title := serviceType
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		fmt.Printf("%s:\n", title)
		for _, info := range infos {
			fmt.Println(info)
		}
		fmt.Println()
	}

	if !confirm(assumeYes, "Would you like to add these services?") {
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

// syncKubernetesServices shows and applies the same sync that serve runs:
// it creates, updates, and deletes discovered services to agree with the annotations.
func syncKubernetesServices(ctx context.Context, db *database.DB, k8s *discovery.KubernetesDiscovery, assumeYes bool) error {
	plan, err := k8s.Plan(ctx, db)
	if err != nil {
		return fmt.Errorf("kubernetes discovery failed: %w", err)
	}
	if plan.Empty() {
		fmt.Println("Kubernetes services are up to date.")
		return nil
	}

	for _, s := range plan.Create {
		fmt.Printf("  + %s (URL: %s)\n", s.InstanceID, s.URL)
	}
	for _, s := range plan.Update {
		fmt.Printf("  ~ %s (URL: %s)\n", s.InstanceID, s.URL)
	}
	for _, id := range plan.Delete {
		fmt.Printf("  - %s\n", id)
	}

	if !confirm(assumeYes, "Would you like to apply these changes?") {
		return nil
	}
	if err := plan.Apply(ctx, db); err != nil {
		return fmt.Errorf("failed to sync Kubernetes services: %w", err)
	}
	fmt.Printf("Kubernetes services synced: %d added, %d updated, %d deleted.\n", len(plan.Create), len(plan.Update), len(plan.Delete))
	return nil
}

// confirm asks a yes/no question, unless assumeYes is set.
func confirm(assumeYes bool, question string) bool {
	if assumeYes {
		fmt.Println("Auto-confirm enabled; applying changes.")
		return true
	}
	fmt.Printf("%s [y/N] ", question)
	var response string
	_, _ = fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Operation cancelled.")
		return false
	}
	return true
}
