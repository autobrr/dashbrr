package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/autobrr"
	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/commands/health"
	"github.com/autobrr/dashbrr/internal/commands/help"
	"github.com/autobrr/dashbrr/internal/commands/omegabrr"
	"github.com/autobrr/dashbrr/internal/commands/service"
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
	dbPath := "./data/dashbrr.db"
	db, err := database.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	registry := base.NewRegistry()
	helpCmd := help.NewHelpCommand(registry)
	serviceCmd := service.NewServiceCommand()

	// Register top-level commands
	registry.Register(version.NewVersionCommand())
	registry.Register(health.NewHealthCommand())
	registry.Register(helpCmd)
	registry.Register(user.NewUserCommand(db))
	registry.Register(serviceCmd)

	// Register service-specific commands
	registry.Register(autobrr.NewAddCommand(db))
	registry.Register(autobrr.NewRemoveCommand(db))
	registry.Register(autobrr.NewListCommand(db))

	// Register omegabrr commands
	registry.Register(omegabrr.NewAddCommand(db))
	registry.Register(omegabrr.NewRemoveCommand(db))
	registry.Register(omegabrr.NewListCommand(db))

	// Set registry on commands that need it
	serviceCmd.SetRegistry(registry)

	// Extract command name and arguments
	cmdName := strings.Join(args[:len(args)-len(args[1:])], " ")
	cmdArgs := args[1:]

	return registry.Execute(context.Background(), cmdName, cmdArgs)
}
