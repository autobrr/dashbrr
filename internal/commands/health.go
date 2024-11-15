package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/general"
	"github.com/autobrr/dashbrr/internal/services/maintainerr"
	"github.com/autobrr/dashbrr/internal/services/omegabrr"
	"github.com/autobrr/dashbrr/internal/services/overseerr"
	"github.com/autobrr/dashbrr/internal/services/plex"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
	"github.com/autobrr/dashbrr/internal/services/tailscale"

	"github.com/spf13/cobra"
)

func HealthCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "health",
		Short: "health",
		Long:  `health`,
		Example: `  dashbrr health
  dashbrr health --help`,
		//SilenceUsage: true,
	}

	var (
		outputJson    = false
		checkUpdate   = false
		checkServices = false
		checkSystem   = false
	)

	command.Flags().BoolVar(&outputJson, "json", false, "output in JSON format")
	command.Flags().BoolVar(&checkUpdate, "check-github", false, "check for updates")
	command.Flags().BoolVar(&checkServices, "checkServices", false, "check checkServices")
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
			// Get all configured services
			services, err := db.GetAllServices(ctx)
			if err != nil {
				// Log error but continue with empty services map
				fmt.Printf("Failed to retrieve checkServices: %v\n", err)
			} else {
				autobrrService := autobrr.NewAutobrrService()
				omegabrrService := omegabrr.NewOmegabrrService()
				radarrService := radarr.NewRadarrService()
				sonarrService := sonarr.NewSonarrService()
				prowlarrService := prowlarr.NewProwlarrService()
				plexService := plex.NewPlexService()
				overseerrService := overseerr.NewOverseerrService()
				maintainerrService := maintainerr.NewMaintainerrService()
				tailscaleService := tailscale.NewTailscaleService()
				generalService := general.NewGeneralService()
				// TODO: Add other services

				for _, service := range services {
					// Check all supported services
					switch {
					case strings.HasPrefix(service.InstanceID, "autobrr-"):
						health, _ := autobrrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "omegabrr-"):
						health, _ := omegabrrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "radarr-"):
						health, _ := radarrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "sonarr-"):
						health, _ := sonarrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "prowlarr-"):
						health, _ := prowlarrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "plex-"):
						health, _ := plexService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "overseerr-"):
						health, _ := overseerrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "maintainerr-"):
						health, _ := maintainerrService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "tailscale-"):
						health, _ := tailscaleService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					case strings.HasPrefix(service.InstanceID, "general-"):
						health, _ := generalService.CheckHealth(ctx, service.URL, service.APIKey)
						status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
					}
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
