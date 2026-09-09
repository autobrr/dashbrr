package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/services/general"
	"github.com/autobrr/dashbrr/internal/types"
)

var generalServiceSpec = serviceSpec{
	Use:         "generic",
	Prefix:      "general-",
	DisplayName: "General",
	Short:       "generic management",
	Long:        "generic management",
	Example:     "  dashbrr service generic\n  dashbrr service generic --help",
	HealthCheck: func(ctx context.Context, serviceURL, apiKey string) (models.ServiceHealth, int) {
		return models.NewGeneralService().CheckHealth(ctx, serviceURL, apiKey)
	},
	HealthOK: func(health models.ServiceHealth) bool {
		return health.Status == "online"
	},
}

// generalDefinitionFlags collects the flags shared by `service generic add`
// and `service generic test` for building a models.CustomServiceConfig.
type generalDefinitionFlags struct {
	authMode    string
	headerName  string
	queryParam  string
	username    string
	password    string
	bearer      string
	healthPath  string
	statusPath  string
	ok          []string
	warn        []string
	versionPath string
	stats       []string
	configFile  string
}

func registerGeneralDefinitionFlags(cmd *cobra.Command, flags *generalDefinitionFlags) {
	cmd.Flags().StringVar(&flags.authMode, "auth-mode", "", "auth mode: none, header, query, basic, bearer")
	cmd.Flags().StringVar(&flags.headerName, "header-name", "", "header name to send the credential in (auth-mode=header)")
	cmd.Flags().StringVar(&flags.queryParam, "query-param", "", "query param to send the credential in (auth-mode=query)")
	cmd.Flags().StringVar(&flags.username, "username", "", "username (auth-mode=basic)")
	cmd.Flags().StringVar(&flags.password, "password", "", "password (auth-mode=basic)")
	cmd.Flags().StringVar(&flags.bearer, "bearer", "", "bearer token (auth-mode=bearer)")
	cmd.Flags().StringVar(&flags.healthPath, "health-path", "", "health check request path")
	cmd.Flags().StringVar(&flags.statusPath, "status-path", "", "gjson path to the status field in the health response body")
	cmd.Flags().StringArrayVar(&flags.ok, "ok", nil, "value at --status-path that means online (repeatable)")
	cmd.Flags().StringArrayVar(&flags.warn, "warn", nil, "value at --status-path that means warning (repeatable)")
	cmd.Flags().StringVar(&flags.versionPath, "version-path", "", "gjson path to the version field in the health response body")
	cmd.Flags().StringArrayVar(&flags.stats, "stat", nil, `a stat to expose, as "Label=json.path[:unit[:format]]" (repeatable)`)
	cmd.Flags().StringVar(&flags.configFile, "config", "", "path to a JSON file with the whole service definition; overrides the other flags")
}

// buildConfig turns the parsed flags into a models.CustomServiceConfig, or
// nil when no relevant flag was set. --config, when set, is authoritative
// and the whole definition is read from that file instead.
func (f *generalDefinitionFlags) buildConfig() (*models.CustomServiceConfig, error) {
	if f.configFile != "" {
		return loadGeneralConfigFile(f.configFile)
	}

	var cfg models.CustomServiceConfig
	hasAny := false

	if f.authMode != "" || f.headerName != "" || f.queryParam != "" || f.username != "" || f.password != "" || f.bearer != "" {
		hasAny = true
		cfg.Auth = &models.CustomAuthConfig{
			Mode:       f.authMode,
			HeaderName: f.headerName,
			QueryParam: f.queryParam,
			Username:   f.username,
			Password:   f.password,
			Token:      f.bearer,
		}
	}

	if f.healthPath != "" || f.statusPath != "" || len(f.ok) > 0 || len(f.warn) > 0 || f.versionPath != "" {
		hasAny = true
		cfg.Health = &models.CustomHealthConfig{
			Path:        f.healthPath,
			StatusPath:  f.statusPath,
			OKValues:    f.ok,
			WarnValues:  f.warn,
			VersionPath: f.versionPath,
		}
	}

	for _, raw := range f.stats {
		stat, err := parseGeneralStatFlag(raw)
		if err != nil {
			return nil, err
		}
		hasAny = true
		cfg.Stats = append(cfg.Stats, stat)
	}

	if !hasAny {
		return nil, nil
	}
	return &cfg, nil
}

func loadGeneralConfigFile(path string) (*models.CustomServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var cfg models.CustomServiceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}
	return &cfg, nil
}

var errInvalidStatFlag = errors.New(`invalid --stat: expected "Label=json.path[:unit[:format]]"`)

// parseGeneralStatFlag parses a single --stat "Label=json.path[:unit[:format]]" value.
func parseGeneralStatFlag(raw string) (models.CustomStatConfig, error) {
	label, rest, ok := strings.Cut(raw, "=")
	if !ok || label == "" || rest == "" {
		return models.CustomStatConfig{}, fmt.Errorf("%w, got %q", errInvalidStatFlag, raw)
	}

	parts := strings.Split(rest, ":")
	if len(parts) > 3 {
		return models.CustomStatConfig{}, fmt.Errorf("%w, got %q", errInvalidStatFlag, raw)
	}

	stat := models.CustomStatConfig{Label: label, Path: parts[0]}
	if len(parts) >= 2 {
		stat.Unit = parts[1]
	}
	if len(parts) >= 3 {
		stat.Format = parts[2]
	}
	return stat, nil
}

// generalTestResult is the shared shape printed by `service generic test`
// and mirrors the POST /api/general/test response.
type generalTestResult struct {
	Status  string
	Version string
	Message string
	Stats   map[string]general.StatValue
	Error   string
}

func runGeneralTest(ctx context.Context, serviceURL, apiKey string, cfg *models.CustomServiceConfig) generalTestResult {
	service := general.NewGeneralService().(*general.GeneralService)

	health, _ := service.Engine.CheckHealth(ctx, serviceURL, apiKey, cfg)
	result := generalTestResult{Status: health.Status, Version: health.Version, Message: health.Message}

	// A non-ok status (e.g. "offline") is a test failure even when
	// FetchStats itself doesn't error - without this, an unreachable or
	// unhealthy service prints "Status: offline" and still exits 0, so a
	// calling script has no way to detect the failure.
	if health.Status != "online" && health.Status != "warning" {
		if health.Message != "" {
			result.Error = health.Message
		} else {
			result.Error = fmt.Sprintf("service reported status %q", health.Status)
		}
		return result
	}

	stats, err := service.FetchStats(ctx, serviceURL, apiKey, cfg)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Stats = stats

	return result
}

func ServiceGeneralCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "generic",
		Short:        generalServiceSpec.Short,
		Long:         generalServiceSpec.Long,
		Example:      generalServiceSpec.Example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	command.AddCommand(ServiceGeneralListCommand())
	command.AddCommand(ServiceGeneralAddCommand())
	command.AddCommand(ServiceGeneralRemoveCommand())
	command.AddCommand(ServiceGeneralTestCommand())

	return command
}

func ServiceGeneralListCommand() *cobra.Command   { return newServiceListCommand(generalServiceSpec) }
func ServiceGeneralRemoveCommand() *cobra.Command { return newServiceRemoveCommand(generalServiceSpec) }

// ServiceGeneralAddCommand is a bespoke `add` (rather than
// newServiceAddCommand) because a general service definition needs the
// auth/health/stat flags above, none of which fit serviceSpec.ParseAdd's
// (url, apiKey, name) shape.
func ServiceGeneralAddCommand() *cobra.Command {
	flags := &generalDefinitionFlags{}

	command := &cobra.Command{
		Use:   "add",
		Short: "add",
		Long:  "add",
		Example: "  dashbrr service generic add [url] [name]\n" +
			"  dashbrr service generic add [url] [name] [apiKey]\n" +
			"  dashbrr service generic add --help",
		Args: cobra.MinimumNArgs(2),
	}

	registerGeneralDefinitionFlags(command, flags)
	var dry bool
	command.Flags().BoolVar(&dry, "dry-run", false, "Dry run, don't write changes")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		serviceURL := args[0]
		displayName := args[1]
		if displayName == "" {
			return errors.New("name is required")
		}
		apiKey := ""
		if len(args) >= 3 {
			apiKey = args[2]
		}

		if _, err := validateHTTPURL(serviceURL); err != nil {
			return err
		}

		cfg, err := flags.buildConfig()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid service definition: %w", err)
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		existing, err := db.FindServiceBy(cmd.Context(), types.FindServiceParams{URL: serviceURL})
		if err != nil {
			return fmt.Errorf("failed to check for existing service: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("service with URL %s already exists", serviceURL)
		}

		// Gate connectivity with the config-aware engine (the same call
		// runGeneralTest makes), not generalServiceSpec.HealthCheck: that
		// helper always drives the nil-config legacy check (GET base URL,
		// apiKey as Bearer), which fails for a definition whose health
		// check lives at a different path and/or uses a different auth
		// mode. cfg is nil here when no flags/--config were given, so the
		// legacy check is exactly what runs for a plain `add`.
		probeService := general.NewGeneralService().(*general.GeneralService)
		health, _ := probeService.Engine.CheckHealth(cmd.Context(), serviceURL, apiKey, cfg)
		if !generalServiceSpec.HealthOK(health) {
			return fmt.Errorf("failed to connect to %s service: %s", generalServiceSpec.DisplayName, health.Message)
		}

		instanceID, err := getNextInstanceID(cmd.Context(), db, generalServiceSpec.Prefix)
		if err != nil {
			return fmt.Errorf("failed to generate instance ID: %w", err)
		}

		service := &models.ServiceConfiguration{
			InstanceID:  instanceID,
			DisplayName: displayName,
			URL:         serviceURL,
			APIKey:      apiKey,
			Config:      cfg,
		}

		if dry {
			fmt.Printf("Dry run: would add %s service:\n", generalServiceSpec.DisplayName)
			fmt.Printf("  URL: %s\n", serviceURL)
			fmt.Printf("  Instance ID: %s\n", instanceID)
			return nil
		}

		if err := db.CreateService(cmd.Context(), service); err != nil {
			return fmt.Errorf("failed to save service configuration: %w", err)
		}

		fmt.Printf("%s service added successfully:\n", generalServiceSpec.DisplayName)
		fmt.Printf("  URL: %s\n", serviceURL)
		if health.Version != "" {
			fmt.Printf("  Version: %s\n", health.Version)
		}
		if health.Status != "" {
			fmt.Printf("  Status: %s\n", health.Status)
		}
		fmt.Printf("  Instance ID: %s\n", instanceID)

		return nil
	}

	return command
}

// ServiceGeneralTestCommand tests a service definition (built the same way
// as `add`, or loaded via --config) without saving it, mirroring
// POST /api/general/test.
func ServiceGeneralTestCommand() *cobra.Command {
	flags := &generalDefinitionFlags{}

	command := &cobra.Command{
		Use:   "test",
		Short: "test a general service definition without saving it",
		Long:  "test a general service definition without saving it",
		Example: "  dashbrr service generic test [url]\n" +
			"  dashbrr service generic test [url] --config definition.json\n" +
			"  dashbrr service generic test [url] [apiKey]",
		Args: cobra.MinimumNArgs(1),
	}

	registerGeneralDefinitionFlags(command, flags)

	command.RunE = func(cmd *cobra.Command, args []string) error {
		serviceURL := args[0]
		apiKey := ""
		if len(args) >= 2 {
			apiKey = args[1]
		}

		if _, err := validateHTTPURL(serviceURL); err != nil {
			return err
		}

		cfg, err := flags.buildConfig()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid service definition: %w", err)
		}

		result := runGeneralTest(cmd.Context(), serviceURL, apiKey, cfg)

		fmt.Printf("Status: %s\n", result.Status)
		if result.Version != "" {
			fmt.Printf("Version: %s\n", result.Version)
		}
		if result.Message != "" {
			fmt.Printf("Message: %s\n", result.Message)
		}
		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}
		if len(result.Stats) > 0 {
			fmt.Println("Stats:")
			labels := make([]string, 0, len(result.Stats))
			for label := range result.Stats {
				labels = append(labels, label)
			}
			sort.Strings(labels)
			for _, label := range labels {
				stat := result.Stats[label]
				if stat.Unit != "" {
					fmt.Printf("  %s: %s %s\n", label, stat.Display, stat.Unit)
					continue
				}
				fmt.Printf("  %s: %s\n", label, stat.Display)
			}
		}

		if result.Error != "" {
			return fmt.Errorf("test failed: %s", result.Error)
		}
		return nil
	}

	return command
}
