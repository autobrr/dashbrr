package commands

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/spf13/cobra"
)

func ServiceAutobrrCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "autobrr",
		Short: "autobrr management",
		Long:  `autobrr torrents`,
		Example: `  dashbrr service autobrr 
  dashbrr service autobrr --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServiceAutobrrListCommand())
	command.AddCommand(ServiceAutobrrAddCommand())
	command.AddCommand(ServiceAutobrrRemoveCommand())

	return command
}

func ServiceAutobrrListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "autobrr",
		Long:    `autobrr`,
		Example: `  dashbrr service list autobrr"
  dashbrr service list autobrr --help`,
		Args: cobra.MinimumNArgs(1),
	}

	var (
		dry     bool
		verbose bool
		dbFile  = ""
	)

	command.Flags().StringVar(&dbFile, "db-file", "torrents.db", "torrents database file. default is ./torrents.db")

	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")
	command.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

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
			fmt.Println("No Autobrr services configured.")
			return nil
		}

		fmt.Println("Configured Autobrr Services:")
		for _, service := range services {

			if strings.HasPrefix(service.InstanceID, "autobrr-") {
				fmt.Printf("  - URL: %s\n", service.URL)
				fmt.Printf("    Instance ID: %s\n", service.InstanceID)

				// Try to get health info which includes version
				autobrrService := autobrr.NewAutobrrService()
				if health, _ := autobrrService.CheckHealth(cmd.Context(), service.URL, service.APIKey); health.Status == "online" {
					fmt.Printf("    Version: %s\n", health.Version)
					fmt.Printf("    Status: %s\n", health.Status)
				}
			}
		}

		return nil
	}

	return command
}

func ServiceAutobrrAddCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "add",
		Short: "add",
		Long:  `add`,
		Example: `  dashbrr service autobrr add"
  dashbrr service autobrr add --help`,
		Args: cobra.MinimumNArgs(2),
	}

	var (
		dry     bool
		verbose bool
		dbFile  = ""
	)

	command.Flags().StringVar(&dbFile, "db-file", "torrents.db", "torrents database file. default is ./torrents.db")

	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")
	command.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

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

		// Create Autobrr service
		autobrrService := autobrr.NewAutobrrService()

		// Perform health check to validate connection
		health, _ := autobrrService.CheckHealth(cmd.Context(), serviceURL, apiKey)

		if health.Status != "online" {
			return fmt.Errorf("failed to connect to Autobrr service: %s", health.Message)
		}

		// Get next available instance ID
		instanceID, err := getNextInstanceID(cmd.Context(), db, "autobrr-")
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %v", err)
		}

		// Create service configuration
		service := &models.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: "Autobrr",
			URL:         serviceURL,
			APIKey:      apiKey,
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %v", err)
		}

		fmt.Printf("Autobrr service added successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Version: %s\n", health.Version)
		fmt.Printf("  Status: %s\n", health.Status)
		fmt.Printf("  Instance ID: %s\n", instanceID)
		return nil
	}

	return command
}

func ServiceAutobrrRemoveCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "remove",
		Short: "remove",
		Long:  `remove`,
		Example: `  dashbrr service autobrr remove"
  dashbrr service autobrr remove --help`,
		Args: cobra.MinimumNArgs(1),
	}

	var (
		dry     bool
		verbose bool
		dbFile  = ""
	)

	command.Flags().StringVar(&dbFile, "db-file", "torrents.db", "torrents database file. default is ./torrents.db")

	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")
	command.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

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

		fmt.Printf("Autobrr service removed successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Instance ID: %s\n", service.InstanceID)

		return nil
	}

	return command
}

func getNextInstanceID(ctx context.Context, db *database.DB, prefix string) (string, error) {
	services, err := db.GetAllServices(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get services: %v", err)
	}

	maxNum := 0
	//prefix := "autobrr-"

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
