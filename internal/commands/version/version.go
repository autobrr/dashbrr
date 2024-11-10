package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/autobrr/dashbrr/internal/commands/base"
)

var (
	version = "dev"
	commit  = ""
	date    = ""

	githubAPIURL = "https://api.github.com/repos/autobrr/dashbrr/releases/latest"
)

type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

type VersionCommand struct {
	*base.BaseCommand
	checkGithub bool
	jsonOutput  bool
}

func NewVersionCommand() *VersionCommand {
	return &VersionCommand{
		BaseCommand: base.NewBaseCommand(
			"version",
			"Display version information",
			"[--check-github] [--json]",
		),
	}
}

func (c *VersionCommand) Execute(ctx context.Context, args []string) error {
	// Parse flags
	for _, arg := range args {
		switch arg {
		case "--check-github":
			c.checkGithub = true
		case "--json":
			c.jsonOutput = true
		}
	}

	// Get current version info
	current := VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	if c.jsonOutput {
		return c.outputJSON(current)
	}

	// Print current version
	fmt.Printf("dashbrr version %s\n", current.Version)
	fmt.Printf("Commit: %s\n", current.Commit)
	fmt.Printf("Built: %s\n", current.Date)

	// Check GitHub if requested
	if c.checkGithub {
		release, err := c.getLatestRelease(ctx)
		if err != nil {
			return fmt.Errorf("failed to check latest version: %w", err)
		}

		fmt.Printf("\nLatest release:\n")
		fmt.Printf("Version: %s\n", release.TagName)
		fmt.Printf("Name: %s\n", release.Name)
		fmt.Printf("Published: %s\n", release.PublishedAt.Format(time.RFC3339))
		fmt.Printf("URL: %s\n", release.HTMLURL)

		if release.TagName != current.Version {
			fmt.Printf("\nUpdate available: %s -> %s\n", current.Version, release.TagName)
		}
	}

	return nil
}

func (c *VersionCommand) getLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "dashbrr/"+version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (c *VersionCommand) outputJSON(info VersionInfo) error {
	type jsonOutput struct {
		Current VersionInfo    `json:"current"`
		Latest  *GitHubRelease `json:"latest,omitempty"`
	}

	output := jsonOutput{
		Current: info,
	}

	if c.checkGithub {
		latest, err := c.getLatestRelease(context.Background())
		if err != nil {
			return err
		}
		output.Latest = latest
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
