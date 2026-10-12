package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestInitializeDatabaseHonoursEnvPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "custom", "dashbrr.db")
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", dbPath)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DASHBRR__CONFIG_PATH", "")

	// Run from a scratch cwd so a regression back to ./data would be visible.
	t.Chdir(dir)

	db, err := initializeDatabase(testCommand(t))
	if err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected database at DASHBRR__DB_PATH %s: %v", dbPath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "dashbrr.db")); !os.IsNotExist(err) {
		t.Fatalf("database was created at the hardcoded ./data path instead of DASHBRR__DB_PATH")
	}
}

func TestInitializeDatabaseDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DASHBRR__DB_TYPE", "")
	t.Setenv("DASHBRR__DB_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DASHBRR__CONFIG_PATH", "")

	t.Chdir(dir)

	db, err := initializeDatabase(testCommand(t))
	if err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, "data", "dashbrr.db")); err != nil {
		t.Fatalf("expected default database at ./data/dashbrr.db: %v", err)
	}
}

// A CLI command must open the database that serve opens for the same --config.
func TestInitializeDatabaseUsesConfigFlag(t *testing.T) {
	t.Setenv("DASHBRR__DB_TYPE", "")
	t.Setenv("DASHBRR__DB_PATH", "")
	t.Chdir(t.TempDir())

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[database]\npath = \"./db/cli.db\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	root := &cobra.Command{Use: "dashbrr"}
	root.PersistentFlags().String("config", "", "")
	root.PersistentFlags().String("db-file", "", "")
	root.AddCommand(&cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := initializeDatabase(cmd)
			if err != nil {
				return err
			}
			return db.Close()
		},
	})
	root.SetArgs([]string{"sub", "--config", configPath})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(configDir, "db", "cli.db")); err != nil {
		t.Fatalf("expected database next to the --config file: %v", err)
	}
}

func testCommand(t *testing.T) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	return cmd
}

// A remove command must only delete a service of its own type, even when the
// URL belongs to a service of another type.
func TestServiceRemoveKeepsServiceOfAnotherType(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", filepath.Join(dir, "dashbrr.db"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DASHBRR__CONFIG_PATH", "")
	t.Chdir(dir)

	db, err := initializeDatabase(testCommand(t))
	if err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}
	defer db.Close()

	const url = "http://media.test:8096"
	if err := db.CreateService(t.Context(), &models.ServiceConfiguration{InstanceID: "jellyfin-1", DisplayName: "Jellyfin", URL: url}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	remove := ServiceEmbyRemoveCommand()
	remove.SetContext(t.Context())
	if err := remove.RunE(remove, []string{url}); err == nil {
		t.Fatal("emby remove with a Jellyfin URL returned no error")
	}

	service, err := db.FindServiceBy(t.Context(), types.FindServiceParams{InstanceID: "jellyfin-1"})
	if err != nil || service == nil {
		t.Fatalf("Jellyfin service is gone after emby remove: service=%v err=%v", service, err)
	}
}

// When two services of different types share a URL, a remove command must
// find and delete the one of its own type.
func TestServiceRemoveFindsOwnTypeWhenURLIsShared(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", filepath.Join(dir, "dashbrr.db"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DASHBRR__CONFIG_PATH", "")
	t.Chdir(dir)

	db, err := initializeDatabase(testCommand(t))
	if err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}
	defer db.Close()

	const url = "http://media.test:8096"
	for _, id := range []string{"jellyfin-1", "emby-1"} {
		if err := db.CreateService(t.Context(), &models.ServiceConfiguration{InstanceID: id, URL: url}); err != nil {
			t.Fatalf("CreateService(%s): %v", id, err)
		}
	}

	remove := ServiceEmbyRemoveCommand()
	remove.SetContext(t.Context())
	if err := remove.RunE(remove, []string{url}); err != nil {
		t.Fatalf("emby remove: %v", err)
	}

	if service, _ := db.FindServiceBy(t.Context(), types.FindServiceParams{InstanceID: "emby-1"}); service != nil {
		t.Fatal("Emby service still exists after emby remove")
	}
	if service, _ := db.FindServiceBy(t.Context(), types.FindServiceParams{InstanceID: "jellyfin-1"}); service == nil {
		t.Fatal("Jellyfin service is gone after emby remove")
	}
}
