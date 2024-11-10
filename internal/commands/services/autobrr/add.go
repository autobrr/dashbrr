package autobrr

import (
	"context"
	"fmt"
	"net/url"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/services/autobrr"
)

type AddCommand struct {
	*base.BaseCommand
}

func NewAddCommand() *AddCommand {
	return &AddCommand{
		BaseCommand: base.NewBaseCommand(
			"autobrr add",
			"Add an Autobrr service configuration",
			"<url> <api-key>",
		),
	}
}

func (c *AddCommand) Execute(ctx context.Context, args []string) error {
	// Validate input arguments
	if len(args) != 2 {
		return fmt.Errorf("insufficient arguments\n\n%s", c.Usage())
	}

	serviceURL := args[0]
	apiKey := args[1]

	// Validate URL
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	// Validate URL scheme and host
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Create Autobrr service
	autobrrService := autobrr.NewAutobrrService()

	// Perform health check to validate connection
	health, _ := autobrrService.CheckHealth(serviceURL, apiKey)

	// Check if service is online
	if health.Status != "online" {
		return fmt.Errorf("failed to connect to Autobrr service: %s", health.Message)
	}

	fmt.Printf("Autobrr service added successfully:\n")
	fmt.Printf("  URL: %s\n", serviceURL)
	fmt.Printf("  Version: %s\n", health.Version)
	fmt.Printf("  Status: %s\n", health.Status)

	return nil
}
