package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
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

	db, err := initializeDatabase(&cobra.Command{})
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

	db, err := initializeDatabase(&cobra.Command{})
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
