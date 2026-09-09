package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeDatabaseHonoursEnvPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "custom", "dashbrr.db")
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", dbPath)

	// Run from a scratch cwd so a regression back to ./data would be visible.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	db, err := initializeDatabase()
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

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	db, err := initializeDatabase()
	if err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, "data", "dashbrr.db")); err != nil {
		t.Fatalf("expected default database at ./data/dashbrr.db: %v", err)
	}
}
