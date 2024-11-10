package autobrr

import (
	"fmt"
)

type RemoveCommand struct{}

func (c *RemoveCommand) Name() string {
	return "remove"
}

func (c *RemoveCommand) Aliases() []string {
	return []string{"delete", "rm"}
}

func (c *RemoveCommand) Description() string {
	return "Remove an Autobrr service configuration"
}

func (c *RemoveCommand) Run(args []string) error {
	// Validate input arguments
	if len(args) != 1 {
		return fmt.Errorf("usage: dashbrr run autobrr remove <url>")
	}

	serviceURL := args[0]

	fmt.Printf("Autobrr service removal not yet implemented:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Println("Note: Service removal functionality will be added in a future update.")

	return nil
}
