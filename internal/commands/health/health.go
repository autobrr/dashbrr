package health

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
	"github.com/autobrr/dashbrr/internal/services/omegabrr"
	"github.com/autobrr/dashbrr/internal/services/prowlarr"
	"github.com/autobrr/dashbrr/internal/services/radarr"
	"github.com/autobrr/dashbrr/internal/services/sonarr"
)

type HealthCommand struct {
	*base.BaseCommand
	checkServices bool
	checkSystem   bool
	jsonOutput    bool
	db            *database.DB
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

func NewHealthCommand(db *database.DB) *HealthCommand {
	return &HealthCommand{
		BaseCommand: base.NewBaseCommand(
			"health",
			"Check system and service health",
			"[--services] [--system] [--json]",
		),
		db: db,
	}
}

func (c *HealthCommand) Execute(ctx context.Context, args []string) error {
	// Parse flags
	for _, arg := range args {
		switch arg {
		case "--services":
			c.checkServices = true
		case "--system":
			c.checkSystem = true
		case "--json":
			c.jsonOutput = true
		}
	}

	// If no specific checks requested, check everything
	if !c.checkServices && !c.checkSystem {
		c.checkServices = true
		c.checkSystem = true
	}

	status := HealthStatus{
		Services: make(map[string]bool),
	}

	// System health checks
	if c.checkSystem {
		// Check database
		if err := c.checkDatabase(&status); err != nil {
			status.System.Database.Error = err.Error()
		}

		// Check config
		if err := c.checkConfig(&status); err != nil {
			status.System.Config.Error = err.Error()
		}
	}

	// Service health checks
	if c.checkServices {
		// Get all configured services
		services, err := c.db.GetAllServices()
		if err != nil {
			// Log error but continue with empty services map
			fmt.Printf("Failed to retrieve services: %v\n", err)
		} else {
			autobrrService := autobrr.NewAutobrrService()
			omegabrrService := omegabrr.NewOmegabrrService()
			radarrService := radarr.NewRadarrService()
			sonarrService := sonarr.NewSonarrService()
			prowlarrService := prowlarr.NewProwlarrService()
			// TODO: Add other services

			for _, service := range services {
				// Check all supported services
				switch {
				case strings.HasPrefix(service.InstanceID, "autobrr-"):
					health, _ := autobrrService.CheckHealth(service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "omegabrr-"):
					health, _ := omegabrrService.CheckHealth(service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "radarr-"):
					health, _ := radarrService.CheckHealth(service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "sonarr-"):
					health, _ := sonarrService.CheckHealth(service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "prowlarr-"):
					health, _ := prowlarrService.CheckHealth(service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				}
			}
		}
	}

	if c.jsonOutput {
		return c.outputJSON(status)
	}

	return c.outputText(status)
}

func (c *HealthCommand) checkDatabase(status *HealthStatus) error {
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

func (c *HealthCommand) checkConfig(status *HealthStatus) error {
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

func (c *HealthCommand) outputJSON(status HealthStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

func (c *HealthCommand) outputText(status HealthStatus) error {
	if c.checkSystem {
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

	if c.checkServices {
		fmt.Println("Service Health:")
		for service, healthy := range status.Services {
			fmt.Printf("  %s: %v\n", service, healthy)
		}
	}

	return nil
}
