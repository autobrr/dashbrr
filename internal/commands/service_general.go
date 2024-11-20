// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/types"

	"github.com/spf13/cobra"
)

func ServiceGeneralCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "generic",
		Short: "generic management",
		Long:  `generic management`,
		Example: `  dashbrr service generic 
  dashbrr service generic --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(ServiceGeneralListCommand())
	command.AddCommand(ServiceGeneralAddCommand())
	command.AddCommand(ServiceGeneralRemoveCommand())

	return command
}

func ServiceGeneralListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "generic",
		Long:    `generic`,
		Example: `  dashbrr service list generic"
  dashbrr service list generic --help`,
		Args: cobra.MinimumNArgs(1),
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

		store, err := initializeCache()
		if err != nil {
			return fmt.Errorf("failed to initialize cache: %v", err)
		}

		allServices, err := db.GetAllServices(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to retrieve services: %v", err)
		}

		if len(allServices) == 0 {
			fmt.Println("No General services configured.")
			return nil
		}

		fmt.Println("Configured General Services:")
		for _, service := range allServices {

			if strings.HasPrefix(service.InstanceID, "general-") {
				fmt.Printf("  - URL: %s\n", service.URL)
				fmt.Printf("    Instance ID: %s\n", service.InstanceID)

				// Try to get health info which includes version
				generalService := services.NewGeneralService(db, store, &service)
				if health, _ := generalService.CheckHealth(cmd.Context(), service.URL, service.APIKey); health.Status == "online" {
					fmt.Printf("    Version: %s\n", health.Version)
					fmt.Printf("    Status: %s\n", health.Status)
				}
			}
		}

		return nil
	}

	return command
}

func ServiceGeneralAddCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "add",
		Short: "add",
		Long:  `add`,
		Example: `  dashbrr service general add [url] [name]
  dashbrr service general add [url] [name] [apiKey]
  dashbrr service general add --help`,
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

		store, err := initializeCache()
		if err != nil {
			return fmt.Errorf("failed to initialize cache: %v", err)
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
		existing, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
		if err != nil {
			return fmt.Errorf("failed to check for existing service: %v", err)
		}
		if existing != nil {
			return fmt.Errorf("service with URL %s already exists", serviceURL)
		}

		// Create General service
		generalService := services.NewGeneralService(db, store, existing)

		// Perform health check to validate connection
		health, _ := generalService.CheckHealth(cmd.Context(), serviceURL, apiKey)

		if health.Status != "online" {
			return fmt.Errorf("failed to connect to General service: %s", health.Message)
		}

		// Get next available instance ID
		instanceID, err := getNextInstanceID(cmd.Context(), db, "general-")
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %v", err)
		}

		// Create service configuration
		service := &types.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: displayName,
			URL:         serviceURL,
			APIKey:      apiKey,
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %v", err)
		}

		fmt.Printf("General service added successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Version: %s\n", health.Version)
		fmt.Printf("  Status: %s\n", health.Status)
		fmt.Printf("  Instance ID: %s\n", instanceID)
		return nil
	}

	return command
}

func ServiceGeneralRemoveCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "remove",
		Short: "remove",
		Long:  `remove`,
		Example: `  dashbrr service general remove"
  dashbrr service general remove --help`,
		Args: cobra.MinimumNArgs(1),
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

		fmt.Printf("General service removed successfully:\n")
		fmt.Printf("  URL: %s\n", serviceURL)
		fmt.Printf("  Instance ID: %s\n", service.InstanceID)

		return nil
	}

	return command
}
