package commands

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/types"
)

func runUserCommand(t *testing.T, args ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "dashbrr", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("config", "", "")
	root.PersistentFlags().String("db-file", "", "")
	root.AddCommand(UserCommand())
	root.SetArgs(append([]string{"user"}, args...))
	return root.ExecuteContext(t.Context())
}

func userTestDB(t *testing.T) func() *database.DB {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DASHBRR__DB_TYPE", "sqlite")
	t.Setenv("DASHBRR__DB_PATH", filepath.Join(dir, "dashbrr.db"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DASHBRR__CONFIG_PATH", "")
	t.Chdir(dir)

	return func() *database.DB {
		t.Helper()
		db, err := initializeDatabase(testCommand(t))
		if err != nil {
			t.Fatalf("initializeDatabase: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
}

func findUser(t *testing.T, db *database.DB, username string) *types.User {
	t.Helper()
	user, err := db.FindUser(t.Context(), types.FindUserParams{Username: username})
	if err != nil {
		t.Fatalf("FindUser(%s): %v", username, err)
	}
	if user == nil {
		t.Fatalf("user %s not found", username)
	}
	return user
}

func TestUserCreate(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantEmail string
	}{
		{name: "default email", args: []string{"admin", "password123"}, wantEmail: "admin@dashbrr.local"},
		{name: "given email", args: []string{"admin", "password123", "admin@example.com"}, wantEmail: "admin@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openDB := userTestDB(t)

			if err := runUserCommand(t, append([]string{"create"}, tt.args...)...); err != nil {
				t.Fatalf("user create %v: %v", tt.args, err)
			}

			user := findUser(t, openDB(), "admin")
			if user.Email != tt.wantEmail {
				t.Errorf("email = %q, want %q", user.Email, tt.wantEmail)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
				t.Errorf("stored hash does not match the given password: %v", err)
			}
		})
	}
}

func TestUserCreateRejectsWrongArgCount(t *testing.T) {
	for _, args := range [][]string{
		{"admin"},
		{"admin", "password123", "admin@example.com", "extra"},
	} {
		userTestDB(t)
		if err := runUserCommand(t, append([]string{"create"}, args...)...); err == nil {
			t.Errorf("user create %v returned no error", args)
		}
	}
}

func TestUserChangePassword(t *testing.T) {
	openDB := userTestDB(t)

	if err := runUserCommand(t, "create", "admin", "password123"); err != nil {
		t.Fatalf("user create: %v", err)
	}
	if err := runUserCommand(t, "change-password", "admin", "newpassword123"); err != nil {
		t.Fatalf("user change-password: %v", err)
	}

	user := findUser(t, openDB(), "admin")
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("newpassword123")); err != nil {
		t.Errorf("stored hash does not match the new password: %v", err)
	}
}

func TestUserChangePasswordUnknownUser(t *testing.T) {
	userTestDB(t)

	if err := runUserCommand(t, "change-password", "nobody", "newpassword123"); err == nil {
		t.Fatal("change-password for an unknown user returned no error")
	}
}
