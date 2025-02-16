// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services"

	"github.com/pkg/errors"
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

		store, err := initializeCache()
		if err != nil {
			return fmt.Errorf("failed to initialize cache: %v", err)
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
			allServices, err := db.GetAllServices(ctx)
			if err != nil {
				// Log error but continue with empty services map
				fmt.Printf("Failed to retrieve checkServices: %v\n", err)
				return errors.Wrap(err, "failed to retrieve services")
			}

			for _, service := range allServices {
				// Check all supported services
				switch {
				case strings.HasPrefix(service.InstanceID, "autobrr-"):
					autobrrService := services.NewAutobrrService(db, store, &service)
					health, _ := autobrrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "radarr-"):
					radarrService := services.NewRadarrService(db, store, &service)
					health, _ := radarrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "sonarr-"):
					sonarrService := services.NewSonarrService(db, store, &service)
					health, _ := sonarrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "prowlarr-"):
					prowlarrService := services.NewProwlarrService(db, store, &service)
					health, _ := prowlarrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "plex-"):
					plexService := services.NewPlexService(db, store, &service)
					health, _ := plexService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "overseerr-"):
					overseerrService := services.NewOverseerrService(db, store, &service)
					health, _ := overseerrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "maintainerr-"):
					maintainerrService := services.NewMaintainerrService(db, store, &service)
					health, _ := maintainerrService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "tailscale-"):
					tailscaleService := services.NewTailscaleService(db, store, &service)
					health, _ := tailscaleService.CheckHealth(ctx, service.URL, service.APIKey)
					status.Services[service.InstanceID] = health.Status == "online" || health.Status == "warning"
				case strings.HasPrefix(service.InstanceID, "general-"):
					generalService := services.NewGeneralService(db, store, &service)
					health, _ := generalService.CheckHealth(ctx, service.URL, service.APIKey)
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
