package health

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/autobrr/dashbrr/internal/commands"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
)

type HealthCommand struct {
	commands.BaseCommand
	checkServices bool
	checkSystem   bool
	jsonOutput    bool
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

func NewHealthCommand() *HealthCommand {
	return &HealthCommand{
		BaseCommand: commands.NewBaseCommand(
			"health",
			"Check system and service health",
			"[--services] [--system] [--json]",
		),
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
		// TODO: Implement service health checks
		// This will be expanded when we add more service-specific checks
		status.Services["autobrr"] = false
		status.Services["plex"] = false
		status.Services["radarr"] = false
		status.Services["sonarr"] = false
	}

	if c.jsonOutput {
		return c.outputJSON(status)
	}

	return c.outputText(status)
}

func (c *HealthCommand) checkDatabase(status *HealthStatus) error {
	// Try to connect to the database
	db, err := database.InitDB("./data/dashbrr.db")
	if err != nil {
		status.System.Database.Connected = false
		status.System.Database.Type = "sqlite3"
		return err
	}
	defer db.Close()

	status.System.Database.Connected = true
	status.System.Database.Type = "sqlite3"
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
