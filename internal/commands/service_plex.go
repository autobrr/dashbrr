package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/spf13/cobra"
)

func ServicePlexCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "plex",
		Short: "plex management",
		Long:  `plex management`,
		Example: `  dashbrr service plex 
  dashbrr service plex --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServicePlexListCommand())
	command.AddCommand(ServicePlexAddCommand())
	command.AddCommand(ServicePlexRemoveCommand())

	return command
}

func ServicePlexListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list",
		Long:    `list`,
		Example: `  dashbrr service plex list"
  dashbrr service plex list --help`,
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
			fmt.Println("No Plex services configured.")
			return nil
		}

		fmt.Println("Configured Plex Services:")
		for _, service := range services {

			if strings.HasPrefix(service.InstanceID, "plex-") {
				fmt.Printf("  - URL: %s\n", service.URL)
				fmt.Printf("    Instance ID: %s\n", service.InstanceID)

				// Try to get health info which includes version
				plexService := plex.NewPlexService()
				if health, _ := plexService.CheckHealth(cmd.Context(), service.URL, service.APIKey); health.Status != "" {
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

func ServicePlexAddCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "add",
		Short: "add",
		Long:  `add`,
		Example: `  dashbrr service plex add http://localhost:5055 <API-KEY>"
  dashbrr service plex add --help`,
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

		// Create Plex service
		plexService := plex.NewPlexService()

		// Perform health check to validate connection
		health, _ := plexService.CheckHealth(cmd.Context(), serviceURL, apiKey)

		if health.Status == "error" || health.Status == "offline" {
			return fmt.Errorf("failed to connect to plex service: %s", health.Message)
		}

		// Get next available instance ID
		instanceID, err := getNextInstanceID(cmd.Context(), db, "plex-")
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %v", err)
		}

		// Create service configuration
		service := &models.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: "Plex",
			URL:         serviceURL,
			APIKey:      apiKey,
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %v", err)
		}

		fmt.Printf("Plex service added successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Version: %s\n", health.Version)
		fmt.Printf("  Status: %s\n", health.Status)
		fmt.Printf("  Instance ID: %s\n", instanceID)

		return nil
	}

	return command
}

func ServicePlexRemoveCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "remove",
		Short: "remove",
		Long:  `remove`,
		Example: `  dashbrr service plex remove"
  dashbrr service plex remove --help`,
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

		fmt.Printf("Plex service removed successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Instance ID: %s\n", service.InstanceID)

		return nil
	}

	return command
}
