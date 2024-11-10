package help

import (
	"context"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
)

// HelpCommand provides help information for commands
type HelpCommand struct {
	*base.BaseCommand
	registry *base.Registry
}

func NewHelpCommand(registry *base.Registry) *HelpCommand {
	return &HelpCommand{
		BaseCommand: base.NewBaseCommand(
			"help",
			"Show help information for commands",
			"[command]\n\n"+
				"Example:\n"+
				"  dashbrr run help\n"+
				"  dashbrr run help service\n"+
				"  dashbrr run help service autobrr\n"+
				"  dashbrr run help service maintainerr\n"+
				"  dashbrr run help service omegabrr\n"+
				"  dashbrr run help service overseerr\n"+
				"  dashbrr run help service plex\n"+
				"  dashbrr run help service general\n"+
				"  dashbrr run help service prowlarr\n"+
				"  dashbrr run help service radarr\n"+
				"  dashbrr run help service sonarr\n"+
				"  dashbrr run help service tailscale\n",
		),
		registry: registry,
	}
}

func (c *HelpCommand) Execute(ctx context.Context, args []string) error {
	var cmdName string
	if len(args) > 0 {
		cmdName = strings.Join(args, " ")
	}

	// Print help information
	println(c.registry.Help(cmdName))
	return nil
}
