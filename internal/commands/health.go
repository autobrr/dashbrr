package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"

	// Service registration (init side effects).
	_ "github.com/autobrr/dashbrr/internal/services/autobrr"
	_ "github.com/autobrr/dashbrr/internal/services/bazarr"
	"github.com/autobrr/dashbrr/internal/services/general"
	_ "github.com/autobrr/dashbrr/internal/services/jellyfin"
	_ "github.com/autobrr/dashbrr/internal/services/lidarr"
	_ "github.com/autobrr/dashbrr/internal/services/maintainerr"
	_ "github.com/autobrr/dashbrr/internal/services/nzbget"
	_ "github.com/autobrr/dashbrr/internal/services/overseerr"
	_ "github.com/autobrr/dashbrr/internal/services/plex"
	_ "github.com/autobrr/dashbrr/internal/services/prowlarr"
	_ "github.com/autobrr/dashbrr/internal/services/qui"
	_ "github.com/autobrr/dashbrr/internal/services/radarr"
	_ "github.com/autobrr/dashbrr/internal/services/readarr"
	_ "github.com/autobrr/dashbrr/internal/services/sabnzbd"
	_ "github.com/autobrr/dashbrr/internal/services/sonarr"
	_ "github.com/autobrr/dashbrr/internal/services/tailscale"
	_ "github.com/autobrr/dashbrr/internal/services/uptimekuma"
	_ "github.com/autobrr/dashbrr/internal/services/whisparr"

	"github.com/spf13/cobra"
)

func HealthCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "health",
		Short: "Check health of system and services",
		Long:  `Check health of system and services`,
		Example: `  dashbrr health --services --system --json
  dashbrr health --help`,
		//SilenceUsage: true,
	}

	var (
		outputJson    = false
		checkServices = false
		checkSystem   = false
	)

	command.Flags().BoolVar(&outputJson, "json", false, "output in JSON format")
	command.Flags().BoolVar(&checkServices, "checkServices", false, "check services")
	command.Flags().BoolVar(&checkSystem, "system", false, "check system")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		// If no specific checks requested, check everything
		if !checkServices && !checkSystem {
			checkServices = true
			checkSystem = true
		}

		status := HealthStatus{
			Services: make(map[string]bool),
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// System health checks
		if checkSystem {
			// Check database
			if err := checkDatabase(&status); err != nil {
				status.System.Database.Error = err.Error()
			}

			// Check config
			if err := checkConfig(&status); err != nil {
				status.System.Config.Error = err.Error()
			}
		}

		ctx := cmd.Context()

		// Service health checks
		if checkServices {
			registry := models.NewServiceRegistry()
			checkers := make(map[string]models.ServiceHealthChecker)

			// Get all configured services
			services, err := db.GetAllServices(ctx)
			if err != nil {
				// Log error but continue with empty services map
				fmt.Printf("Failed to retrieve checkServices: %v\n", err)
			} else {
				for _, service := range services {
					serviceType, _, _ := strings.Cut(service.InstanceID, "-")
					if serviceType == "" {
						continue
					}

					checker := checkers[serviceType]
					if checker == nil {
						checker = registry.CreateService(serviceType)
						if checker == nil {
							continue
						}
						checkers[serviceType] = checker
					}

					health, _ := checkServiceHealth(ctx, checker, serviceType, service)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				}
			}
		}

		if outputJson {
			return outputJSON(status)
		} else {
			healthOutputText(checkSystem, checkServices, status)
		}

		//return outputText(status)

		return nil
	}

	return command
}

// checkServiceHealth runs the health check for one configured service. For a
// "general" (custom) instance with a stored CustomServiceConfig, it drives
// the config-aware engine directly - the same pattern used by the `service
// generic add`/`test` connectivity gate - instead of the registry's 3-arg
// models.ServiceHealthChecker interface, which cannot carry service.Config
// and always falls back to the nil-config legacy probe (GET the base URL,
// apiKey as Bearer). Every other service type, and a general instance with
// no config yet, keeps using the registry's checker unchanged.
func checkServiceHealth(ctx context.Context, checker models.ServiceHealthChecker, serviceType string, service models.ServiceConfiguration) (models.ServiceHealth, int) {
	if serviceType == "general" && service.Config != nil {
		if generalChecker, ok := checker.(*general.GeneralService); ok {
			return generalChecker.Engine.CheckHealth(ctx, service.URL, service.APIKey, service.Config)
		}
	}

	return checker.CheckHealth(ctx, service.URL, service.APIKey)
}

func checkDatabase(status *HealthStatus) error {
	// Get database configuration
	dbConfig := database.NewConfig()
	status.System.Database.Type = dbConfig.Driver

	// Try to connect to the database
	var db *database.DB
	var err error

	// Connect using config regardless of driver type
	db, err = database.InitDBWithConfig(dbConfig)

	if err != nil {
		status.System.Database.Connected = false
		return err
	}
	defer db.Close()

	status.System.Database.Connected = true
	return nil
}

func checkConfig(status *HealthStatus) error {
	_, err := config.LoadConfig("config.toml")
	if err != nil {
		status.System.Config.Valid = false
		status.System.Config.Path = "config.toml"
		return err
	}

	status.System.Config.Valid = true
	status.System.Config.Path = "config.toml"
	return nil
}

func healthOutputText(checkSystem, checkServices bool, status HealthStatus) error {
	if checkSystem {
		fmt.Println("System Health:")
		fmt.Printf("  Database:\n")
		fmt.Printf("    Connected: %v\n", status.System.Database.Connected)
		fmt.Printf("    Type: %s\n", status.System.Database.Type)
		if status.System.Database.Error != "" {
			fmt.Printf("    Error: %s\n", status.System.Database.Error)
		}

		fmt.Printf("\n  Config:\n")
		fmt.Printf("    Valid: %v\n", status.System.Config.Valid)
		fmt.Printf("    Path: %s\n", status.System.Config.Path)
		if status.System.Config.Error != "" {
			fmt.Printf("    Error: %s\n", status.System.Config.Error)
		}
		fmt.Println()
	}

	if checkServices {
		fmt.Println("Service Health:")
		for service, healthy := range status.Services {
			fmt.Printf("  %s: %v\n", service, healthy)
		}
	}

	return nil
}

func outputJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

type HealthStatus struct {
	System struct {
		Database struct {
			Connected bool   `json:"connected"`
			Type      string `json:"type"`
			Error     string `json:"error,omitempty"`
		} `json:"database"`
		Config struct {
			Valid bool   `json:"valid"`
			Path  string `json:"path"`
			Error string `json:"error,omitempty"`
		} `json:"config"`
	} `json:"system"`
	Services map[string]bool `json:"services,omitempty"`
}
