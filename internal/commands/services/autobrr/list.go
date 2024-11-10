package autobrr

import (
	"context"
	"fmt"

	"github.com/autobrr/dashbrr/internal/commands/base"
)

type ListCommand struct {
	*base.BaseCommand
}

func NewListCommand() *ListCommand {
	return &ListCommand{
		BaseCommand: base.NewBaseCommand(
			"autobrr list",
			"List configured Autobrr services",
			"",
		),
	}
}

func (c *ListCommand) Execute(ctx context.Context, args []string) error {
	fmt.Println("Autobrr service listing not yet implemented.")
	fmt.Println("Note: Service listing functionality will be added in a future update.")

	return nil
}
