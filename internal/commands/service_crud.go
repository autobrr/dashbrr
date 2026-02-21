package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/spf13/cobra"
)

type serviceSpec struct {
	Use         string
	Prefix      string
	DisplayName string
	Short       string
	Long        string
	Example     string

	AddExample string
	AddArgs    cobra.PositionalArgs
	ParseAdd   func(args []string) (serviceURL, apiKey, displayName string, err error)

	HealthCheck func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int)
	HealthOK    func(health models.ServiceHealth) bool
}

func defaultHealthOK(health models.ServiceHealth) bool {
	if health.Status == "" {
		return false
	}
	return health.Status != "error" && health.Status != "offline"
}

func newServiceListCommand(spec serviceSpec) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list",
		Long:    "list",
		Example: fmt.Sprintf("  dashbrr service %s list\n  dashbrr service %s list --help", spec.Use, spec.Use),
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := initializeDatabase()
			if err != nil {
				return fmt.Errorf("failed to initialize database: %v", err)
			}

			services, err := db.GetAllServices(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to retrieve services: %v", err)
			}

			matches := make([]models.ServiceConfiguration, 0, len(services))
			for _, service := range services {
				if strings.HasPrefix(service.InstanceID, spec.Prefix) {
					matches = append(matches, service)
				}
			}

			if len(matches) == 0 {
				fmt.Printf("No %s services configured.\n", spec.DisplayName)
				return nil
			}

			fmt.Printf("Configured %s Services:\n", spec.DisplayName)
			for _, service := range matches {
				fmt.Printf("  - URL: %s\n", service.URL)
				fmt.Printf("    Instance ID: %s\n", service.InstanceID)

				if spec.HealthCheck == nil {
					continue
				}
				health, _ := spec.HealthCheck(cmd.Context(), service.URL, service.APIKey)
				if health.Version != "" {
					fmt.Printf("    Version: %s\n", health.Version)
				}
				if health.Status != "" {
					fmt.Printf("    Status: %s\n", health.Status)
				}
			}

			return nil
		},
	}
}

func newServiceAddCommand(spec serviceSpec) *cobra.Command {
	command := &cobra.Command{
		Use:     "add",
		Short:   "add",
		Long:    "add",
		Example: spec.AddExample,
		Args:    spec.AddArgs,
	}

	var dry bool
	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		if spec.ParseAdd == nil {
			return fmt.Errorf("service %s: ParseAdd not configured", spec.Use)
		}

		serviceURL, apiKey, displayName, err := spec.ParseAdd(args)
		if err != nil {
			return err
		}

		if _, err := validateHTTPURL(serviceURL); err != nil {
			return err
		}

		existing, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
		if err != nil {
			return fmt.Errorf("failed to check for existing service: %v", err)
		}
		if existing != nil {
			return fmt.Errorf("service with URL %s already exists", serviceURL)
		}

		if spec.HealthCheck != nil {
			health, _ := spec.HealthCheck(cmd.Context(), serviceURL, apiKey)
			ok := defaultHealthOK(health)
			if spec.HealthOK != nil {
				ok = spec.HealthOK(health)
			}
			if !ok {
				return fmt.Errorf("failed to connect to %s service: %s", spec.DisplayName, health.Message)
			}
		}

		instanceID, err := getNextInstanceID(cmd.Context(), db, spec.Prefix)
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %v", err)
		}

		service := &models.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: displayName,
			URL:         serviceURL,
			APIKey:      apiKey,
		}

		if dry {
			fmt.Printf("Dry run: would add %s service:\n", spec.DisplayName)
			fmt.Printf("  URL: %s\n", serviceURL)
			fmt.Printf("  Instance ID: %s\n", instanceID)
			return nil
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %v", err)
		}

		fmt.Printf("%s service added successfully:\n", spec.DisplayName)
		fmt.Printf("  URL: %s\n", serviceURL)
		if spec.HealthCheck != nil {
			health, _ := spec.HealthCheck(cmd.Context(), serviceURL, apiKey)
			if health.Version != "" {
				fmt.Printf("  Version: %s\n", health.Version)
			}
			if health.Status != "" {
				fmt.Printf("  Status: %s\n", health.Status)
			}
		}
		fmt.Printf("  Instance ID: %s\n", instanceID)

		return nil
	}

	return command
}

func newServiceRemoveCommand(spec serviceSpec) *cobra.Command {
	return &cobra.Command{
		Use:     "remove",
		Short:   "remove",
		Long:    "remove",
		Example: fmt.Sprintf("  dashbrr service %s remove <URL>\n  dashbrr service %s remove --help", spec.Use, spec.Use),
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := initializeDatabase()
			if err != nil {
				return fmt.Errorf("failed to initialize database: %v", err)
			}

			serviceURL := args[0]
			service, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
			if err != nil {
				return fmt.Errorf("failed to find service: %v", err)
			}
			if service == nil {
				return fmt.Errorf("no service found with URL: %s", serviceURL)
			}

			if err := db.DeleteService(cmd.Context(), service.InstanceID); err != nil {
				return fmt.Errorf("failed to remove service: %v", err)
			}

			fmt.Printf("%s service removed successfully:\n", spec.DisplayName)
			fmt.Printf("  URL: %s\n", serviceURL)
			fmt.Printf("  Instance ID: %s\n", service.InstanceID)

			return nil
		},
	}
}
