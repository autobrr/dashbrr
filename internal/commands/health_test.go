package commands

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autobrr/dashbrr/internal/database"
)

func runHealth(t *testing.T, args ...string) error {
	t.Helper()
	cmd := HealthCommand()
	cmd.SetContext(t.Context())
	cmd.SetArgs(append([]string{"--system"}, args...))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

func isolateHealthEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DASHBRR__DB_TYPE", "")
	t.Setenv("DASHBRR__DB_PATH", "")
	t.Setenv("DASHBRR__CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(dir)
	return dir
}

// Health is a probe: in an empty directory it reports the missing database
// and creates nothing.
func TestHealthEmptyDirectoryWritesNothing(t *testing.T) {
	dir := isolateHealthEnv(t)

	err := runHealth(t, "--json")
	if err == nil || !strings.Contains(err.Error(), "database not found") {
		t.Fatalf("health returned %v, want a database not found error", err)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		t.Errorf("health created %s", e.Name())
	}
}

func TestHealthEnvDatabaseWithoutConfigFile(t *testing.T) {
	dir := isolateHealthEnv(t)
	dbPath := filepath.Join(t.TempDir(), "dashbrr.db")
	t.Setenv("DASHBRR__DB_PATH", dbPath)

	db, err := database.InitDBWithConfig(&database.Config{Driver: "sqlite", Path: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := runHealth(t); err != nil {
		t.Fatalf("health: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("health wrote config.toml")
	}
}
