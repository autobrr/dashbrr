package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/spf13/cobra"
)

func ServiceRadarrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "radarr",
		Short: "radarr management",
		Long:  `radarr management`,
		Example: `  dashbrr service radarr 
  dashbrr service radarr --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServiceRadarrListCommand())
	command.AddCommand(ServiceRadarrAddCommand())
	command.AddCommand(ServiceRadarrRemoveCommand())

	return command
}

func ServiceRadarrListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list",
		Long:    `list`,
		Example: `  dashbrr service radarr list"
  dashbrr service radarr list --help`,
		//Args: cobra.MinimumNArgs(1),
	}

	//var (
	//	dry     bool
	//)
	//
	//command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		services, err := db.GetAllServices(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to retrieve services: %v", err)
		}

		if len(services) == 0 {
			fmt.Println("No Radarr services configured.")
			return nil
		}

		fmt.Println("Configured Radarr Services:")
		for _, service := range services {

			if strings.HasPrefix(service.InstanceID, "radarr-") {
				fmt.Printf("  - URL: %s\n", service.URL)
				fmt.Printf("    Instance ID: %s\n", service.InstanceID)

				// Try to get health info which includes version
				radarrService := radarr.NewRadarrService()
				if health, _ := radarrService.CheckHealth(cmd.Context(), service.URL, service.APIKey); health.Status != "" {
					if health.Version != "" {
						fmt.Printf("    Version: %s\n", health.Version)
					}
					fmt.Printf("    Status: %s\n", health.Status)
				}
			}
		}

		return nil
	}

	return command
}

func ServiceRadarrAddCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "add",
		Short: "add",
		Long:  `add`,
		Example: `  dashbrr service radarr add http://localhost:7878 <API-KEY>"
  dashbrr service radarr add --help`,
		Args: cobra.MinimumNArgs(2),
	}

	var (
		dry bool
	)

	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
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
		existing, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
		if err != nil {
			return fmt.Errorf("failed to check for existing service: %v", err)
		}
		if existing != nil {
			return fmt.Errorf("service with URL %s already exists", serviceURL)
		}

		// Create Radarr service
		radarrService := radarr.NewRadarrService()

		// Perform health check to validate connection
		health, _ := radarrService.CheckHealth(cmd.Context(), serviceURL, apiKey)

		if health.Status == "error" || health.Status == "offline" {
			return fmt.Errorf("failed to connect to radarr service: %s", health.Message)
		}

		// Get next available instance ID
		instanceID, err := getNextInstanceID(cmd.Context(), db, "radarr-")
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %v", err)
		}

		// Create service configuration
		service := &models.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: "Radarr",
			URL:         serviceURL,
			APIKey:      apiKey,
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %v", err)
		}

		fmt.Printf("Radarr service added successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Version: %s\n", health.Version)
		fmt.Printf("  Status: %s\n", health.Status)
		fmt.Printf("  Instance ID: %s\n", instanceID)

		return nil
	}

	return command
}

func ServiceRadarrRemoveCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "remove",
		Short: "remove",
		Long:  `remove`,
		Example: `  dashbrr service radarr remove"
  dashbrr service radarr remove --help`,
		Args: cobra.MinimumNArgs(1),
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		serviceURL := args[0]

		// Find service by URL
		service, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
		if err != nil {
			return fmt.Errorf("failed to find service: %v", err)
		}
		if service == nil {
			return fmt.Errorf("no service found with URL: %s", serviceURL)
		}

		// Delete service
		if err := db.DeleteService(cmd.Context(), service.InstanceID); err != nil {
			return fmt.Errorf("failed to remove service: %v", err)
		}

		fmt.Printf("Radarr service removed successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Instance ID: %s\n", service.InstanceID)

		return nil
	}

	return command
}
