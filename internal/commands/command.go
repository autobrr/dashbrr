package commands

import (
	"context"
	"fmt"
)

// Command represents a CLI command that can be executed
type Command interface {
	// Execute runs the command with the given arguments
	Execute(ctx context.Context, args []string) error
	// Name returns the name of the command
	Name() string
	// Description returns a brief description of what the command does
	Description() string
	// Usage returns detailed usage instructions
	Usage() string
}

// BaseCommand provides common functionality for commands
type BaseCommand struct {
	name        string
	description string
	usage       string
}

// Name returns the command name
func (c *BaseCommand) Name() string {
	return c.name
}

// Description returns the command description
func (c *BaseCommand) Description() string {
	return c.description
}

// Usage returns command usage instructions
func (c *BaseCommand) Usage() string {
	return fmt.Sprintf("Usage: dashbrr run %s %s", c.name, c.usage)
}

// NewBaseCommand creates a new base command
func NewBaseCommand(name, description, usage string) BaseCommand {
	return BaseCommand{
		name:        name,
		description: description,
		usage:       usage,
	}
}
