package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/commands/base"
)

// ServiceCommand is the top-level command for managing services
type ServiceCommand struct {
	*base.BaseCommand
	registry *base.Registry
}

func NewServiceCommand() *ServiceCommand {
	return &ServiceCommand{
		BaseCommand: base.NewBaseCommand(
			"service",
			"Manage service configurations",
			"<service-type> <action> [arguments]\n\n"+
				"  Service Types:\n"+
				"    autobrr    - Autobrr service management\n"+
				"    omegabrr   - Omegabrr service management\n\n"+
				"  Use 'dashbrr run help service <service-type>' for more information",
		),
	}
}

// SetRegistry allows setting the registry after command creation
func (c *ServiceCommand) SetRegistry(registry *base.Registry) {
	c.registry = registry
}

func (c *ServiceCommand) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no service type specified\n\n%s", c.Usage())
	}

	if c.registry == nil {
		return fmt.Errorf("command registry not initialized")
	}

	serviceType := args[0]
	if len(args) == 1 {
		// Let the help system handle showing available commands
		return fmt.Errorf("no action specified for %s service\n\n%s",
			serviceType,
			c.registry.Help("service "+serviceType))
	}

	// Reconstruct the full command name (e.g., "service autobrr add")
	fullCmd := strings.Join(append([]string{"service", serviceType, args[1]}), " ")

	// Look up the command in the registry
	cmd, exists := c.registry.Get(fullCmd)
	if !exists {
		// Let the help system handle showing available commands
		return fmt.Errorf("unknown action '%s' for service %s\n\n%s",
			args[1],
			serviceType,
			c.registry.Help("service "+serviceType))
	}

	// Execute the command with the remaining arguments
	return cmd.Execute(ctx, args[2:])
}
