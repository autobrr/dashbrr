package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/autobrr"
	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/commands/general"
	"github.com/autobrr/dashbrr/internal/commands/health"
	"github.com/autobrr/dashbrr/internal/commands/help"
	"github.com/autobrr/dashbrr/internal/commands/maintainerr"
	"github.com/autobrr/dashbrr/internal/commands/omegabrr"
	"github.com/autobrr/dashbrr/internal/commands/overseerr"
	"github.com/autobrr/dashbrr/internal/commands/plex"
	"github.com/autobrr/dashbrr/internal/commands/prowlarr"
	"github.com/autobrr/dashbrr/internal/commands/radarr"
	"github.com/autobrr/dashbrr/internal/commands/service"
	"github.com/autobrr/dashbrr/internal/commands/sonarr"
	"github.com/autobrr/dashbrr/internal/commands/tailscale"
	"github.com/autobrr/dashbrr/internal/commands/user"
	"github.com/autobrr/dashbrr/internal/commands/version"
	"github.com/autobrr/dashbrr/internal/database"
)

// ExecuteCommand handles the execution of CLI commands
func ExecuteCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no command specified\n\nRun 'dashbrr run help' for usage")
	}

	// Initialize database for commands
	db, err := initializeDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	registry := base.NewRegistry()

	// Register commands
	if err := registerCommands(registry, db); err != nil {
		return err
	}

	// Extract command name and arguments
	cmdName := strings.Join(args[:len(args)-len(args[1:])], " ")
	cmdArgs := args[1:]

	return registry.Execute(context.Background(), cmdName, cmdArgs)
}

func initializeDatabase() (*database.DB, error) {
	dbPath := "./data/dashbrr.db"
	db, err := database.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}
	return db, nil
}

func registerCommands(registry *base.Registry, db *database.DB) error {
	// Create commands that need special handling
	helpCmd := help.NewHelpCommand(registry)
	serviceCmd := service.NewServiceCommand()

	// Register top-level commands
	topLevelCommands := []base.Command{
		version.NewVersionCommand(),
		health.NewHealthCommand(db),
		helpCmd,
		user.NewUserCommand(db),
		serviceCmd,
	}

	serviceCommands := []base.Command{
		autobrr.NewAddCommand(db), autobrr.NewRemoveCommand(db), autobrr.NewListCommand(db),
		omegabrr.NewAddCommand(db), omegabrr.NewRemoveCommand(db), omegabrr.NewListCommand(db),
		radarr.NewAddCommand(db), radarr.NewRemoveCommand(db), radarr.NewListCommand(db),
		sonarr.NewAddCommand(db), sonarr.NewRemoveCommand(db), sonarr.NewListCommand(db),
		prowlarr.NewAddCommand(db), prowlarr.NewRemoveCommand(db), prowlarr.NewListCommand(db),
		plex.NewAddCommand(db), plex.NewRemoveCommand(db), plex.NewListCommand(db),
		overseerr.NewAddCommand(db), overseerr.NewRemoveCommand(db), overseerr.NewListCommand(db),
		maintainerr.NewAddCommand(db), maintainerr.NewRemoveCommand(db), maintainerr.NewListCommand(db),
		tailscale.NewAddCommand(db), tailscale.NewRemoveCommand(db), tailscale.NewListCommand(db),
		general.NewAddCommand(db), general.NewRemoveCommand(db), general.NewListCommand(db),
	}

	// Register all commands
	for _, cmd := range append(topLevelCommands, serviceCommands...) {
		registry.Register(cmd)
	}

	// Set registry on commands that need it
	serviceCmd.SetRegistry(registry)

	return nil
}
