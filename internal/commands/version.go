package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/autobrr/dashbrr/internal/buildinfo"

	"github.com/spf13/cobra"
)

const githubAPIURL = "https://api.github.com/repos/autobrr/dashbrr/releases/latest"

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

func VersionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "version",
		Short: "version",
		Long:  `version`,
		Example: `  dashbrr version
  dashbrr version --help`,
		//SilenceUsage: true,
	}

	var (
		outputJson  = false
		checkUpdate = false
	)

	command.Flags().BoolVar(&outputJson, "json", false, "output in JSON format")
	command.Flags().BoolVar(&checkUpdate, "check-github", false, "check for updates")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		// Get current version info
		current := VersionInfo{
			Version: buildinfo.Version,
			Commit:  buildinfo.Commit,
			Date:    buildinfo.Date,
		}

		if outputJson {
			return versionOutputJSON(checkUpdate, current)
		}

		// Print current version
		fmt.Printf("dashbrr version %s\n", current.Version)
		fmt.Printf("Commit: %s\n", current.Commit)
		fmt.Printf("Built: %s\n", current.Date)

		// Check GitHub if requested
		if checkUpdate {
			release, err := getLatestRelease(cmd.Context())
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

	return command
}

func getLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	buildinfo.AttachUserAgentHeader(req)

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

func versionOutputJSON(check bool, info VersionInfo) error {
	type jsonOutput struct {
		Current VersionInfo    `json:"current"`
		Latest  *GitHubRelease `json:"latest,omitempty"`
	}

	output := jsonOutput{
		Current: info,
	}

	if check {
		latest, err := getLatestRelease(context.Background())
		if err != nil {
			return err
		}
		output.Latest = latest
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
