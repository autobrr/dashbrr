package commands

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
		names = append(names, name)
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

	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Sprintf("Unknown command: %s\n\n%s", name, r.ListCommands())
	}

	return fmt.Sprintf("%s\n\n%s\n", cmd.Description(), cmd.Usage())
}
