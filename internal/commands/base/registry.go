package base

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Registry manages the available commands
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// Get returns a command by name
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Execute runs a command by name with the given arguments
func (r *Registry) Execute(ctx context.Context, name string, args []string) error {
	// Split command name into parts (e.g., "service autobrr add" -> ["service", "autobrr", "add"])
	parts := strings.Split(name, " ")
	if len(parts) > 1 {
		// Check for subcommand
		fullName := strings.Join(parts, " ")
		if cmd, ok := r.Get(fullName); ok {
			return cmd.Execute(ctx, args)
		}
	}

	// Check for main command
	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("unknown command: %s\n\nRun 'dashbrr run help' for usage", name)
	}

	return cmd.Execute(ctx, args)
}

// ListCommands returns a sorted list of available commands with their descriptions
func (r *Registry) ListCommands() string {
	var names []string
	for name := range r.commands {
		// Only include top-level commands in the main list
		if !strings.Contains(name, " ") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Usage: dashbrr run <command> [arguments]\n\n")
	b.WriteString("Available commands:\n\n")

	for _, name := range names {
		cmd := r.commands[name]
		b.WriteString(fmt.Sprintf("  %-12s %s\n", name, cmd.Description()))
	}

	b.WriteString("\nUse 'dashbrr run help <command>' for more information about a command.")
	return b.String()
}

// Help returns detailed help for a specific command
func (r *Registry) Help(name string) string {
	if name == "" {
		return r.ListCommands()
	}

	// Check if this is a request for service subcommands
	if name == "service" {
		return r.listServiceCommands()
	}

	// Check if this is a request for config subcommands
	if name == "config" {
		return r.listConfigCommands()
	}

	// Check if this is a request for specific service type commands
	if strings.HasPrefix(name, "service ") {
		parts := strings.Split(name, " ")
		if len(parts) == 2 {
			return r.listServiceTypeCommands(parts[1])
		}
		// If more parts exist, try to get help for the specific command
		if len(parts) > 2 {
			fullCmd := strings.Join(parts, " ")
			if cmd, ok := r.Get(fullCmd); ok {
				return fmt.Sprintf("%s\n\n%s\n", cmd.Description(), cmd.Usage())
			}
		}
	}

	// Check if this is a request for specific config subcommands
	if strings.HasPrefix(name, "config ") {
		parts := strings.Split(name, " ")
		if len(parts) == 2 {
			return r.listConfigSubcommandHelp(parts[1])
		}
	}

	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Sprintf("Unknown command: %s\n\n%s", name, r.ListCommands())
	}

	return fmt.Sprintf("%s\n\n%s\n", cmd.Description(), cmd.Usage())
}

// listServiceCommands returns help for all available service types
func (r *Registry) listServiceCommands() string {
	var b strings.Builder
	b.WriteString("Usage: dashbrr run service <service-type> <action> [arguments]\n\n")
	b.WriteString("Available service types:\n\n")
	b.WriteString("  autobrr    - Autobrr service management\n")
	b.WriteString("  omegabrr   - Omegabrr service management\n")
	b.WriteString("  radarr     - Radarr service management\n")
	b.WriteString("  sonarr     - Sonarr service management\n")
	b.WriteString("  prowlarr   - Prowlarr service management\n")
	b.WriteString("\nUse 'dashbrr run help service <service-type>' for more information about a service type.")
	return b.String()
}

// listConfigCommands returns help for all available config commands
func (r *Registry) listConfigCommands() string {
	var b strings.Builder
	b.WriteString("Usage: dashbrr run config <subcommand> [arguments]\n\n")
	b.WriteString("Available subcommands:\n\n")
	b.WriteString("  discover   - Discover services from Docker/Kubernetes labels\n")
	b.WriteString("  import     - Import services from external config file\n")
	b.WriteString("  export     - Export current service configurations\n")
	b.WriteString("\nExamples:\n\n")
	b.WriteString("  # Discover services from Docker labels\n")
	b.WriteString("  dashbrr run config discover --docker\n\n")
	b.WriteString("  # Import services from external config\n")
	b.WriteString("  dashbrr run config import services.yaml\n\n")
	b.WriteString("  # Export configurations\n")
	b.WriteString("  dashbrr run config export --format=yaml --mask-secrets --output=services.yaml\n")
	b.WriteString("\nUse 'dashbrr run help config <subcommand>' for more information about a subcommand.")
	return b.String()
}

// listConfigSubcommandHelp returns detailed help for a specific config subcommand
func (r *Registry) listConfigSubcommandHelp(subcommand string) string {
	var b strings.Builder

	switch subcommand {
	case "discover":
		b.WriteString("Usage: dashbrr run config discover [--docker] [--k8s]\n\n")
		b.WriteString("Discover services from Docker labels and/or Kubernetes.\n\n")
		b.WriteString("Options:\n")
		b.WriteString("  --docker    Discover services from Docker labels\n")
		b.WriteString("  --k8s       Discover services from Kubernetes\n\n")
		b.WriteString("If no options are specified, both Docker and Kubernetes discovery will be attempted.\n")

	case "import":
		b.WriteString("Usage: dashbrr run config import <file>\n\n")
		b.WriteString("Import services from an external configuration file.\n\n")
		b.WriteString("Arguments:\n")
		b.WriteString("  <file>      Path to the configuration file (YAML or JSON)\n")

	case "export":
		b.WriteString("Usage: dashbrr run config export [--format=<yaml|json>] [--mask-secrets] [--output=<file>]\n\n")
		b.WriteString("Export current service configurations to a file.\n\n")
		b.WriteString("Options:\n")
		b.WriteString("  --format=<yaml|json>    Output format (default: yaml)\n")
		b.WriteString("  --mask-secrets          Mask sensitive information like API keys\n")
		b.WriteString("  --output=<file>         Output file path (default: dashbrr-services.<format>)\n")

	default:
		return fmt.Sprintf("Unknown config subcommand: %s\n\nRun 'dashbrr run help config' for available subcommands.", subcommand)
	}

	return b.String()
}

// listServiceTypeCommands returns help for a specific service type's commands
func (r *Registry) listServiceTypeCommands(serviceType string) string {
	prefix := "service " + serviceType + " "
	var commands []struct {
		name        string
		description string
	}

	for name, cmd := range r.commands {
		if strings.HasPrefix(name, prefix) {
			action := strings.TrimPrefix(name, prefix)
			commands = append(commands, struct {
				name        string
				description string
			}{
				name:        action,
				description: cmd.Description(),
			})
		}
	}

	if len(commands) == 0 {
		return fmt.Sprintf("Unknown service type: %s\n\nRun 'dashbrr run help service' for a list of service types.", serviceType)
	}

	// Sort commands by name
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].name < commands[j].name
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Usage: dashbrr run service %s <action> [arguments]\n\n", serviceType))
	b.WriteString("Available actions:\n\n")

	for _, cmd := range commands {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", cmd.name, cmd.description))
	}

	b.WriteString(fmt.Sprintf("\nUse 'dashbrr run help service %s <action>' for more information about an action.", serviceType))
	return b.String()
}
