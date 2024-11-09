package help

import (
	"context"
	"fmt"

	"github.com/autobrr/dashbrr/internal/commands"
)

type HelpCommand struct {
	commands.BaseCommand
	registry *commands.Registry
}

func NewHelpCommand(registry *commands.Registry) *HelpCommand {
	return &HelpCommand{
		BaseCommand: commands.NewBaseCommand(
			"help",
			"Show help about available commands",
			"[command]",
		),
		registry: registry,
	}
}

func (c *HelpCommand) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Println(c.registry.ListCommands())
		return nil
	}

	fmt.Println(c.registry.Help(args[0]))
	return nil
}

// SetRegistry allows setting the registry after command creation
// This is useful when the help command needs to be registered in the registry itself
func (c *HelpCommand) SetRegistry(registry *commands.Registry) {
	c.registry = registry
}
