package commands

import (
	"io"
	"testing"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/types"
)

// Regression test for the args off-by-one: `user create <username> <password>`
// must read username from args[0] and password from args[1] (not args[1]/args[2],
// which panicked on a 2-arg invocation).
func TestUserCreateArgs(t *testing.T) {
	t.Chdir(t.TempDir())

	cmd := UserCreateCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"someuser", "some-password"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected user create with 2 args to succeed, got: %v", err)
	}

	db, err := database.InitDB("./data/dashbrr.db")
	if err != nil {
		t.Fatalf("failed to open persisted database: %v", err)
	}
	defer db.Close()

	user, err := db.FindUser(t.Context(), types.FindUserParams{Username: "someuser"})
	if err != nil {
		t.Fatalf("FindUser: %v", err)
	}
	if user == nil {
		t.Fatal("expected the created user to be found")
	}
	if user.Username != "someuser" {
		t.Fatalf("Username = %q, want %q", user.Username, "someuser")
	}
	if user.Email != "someuser@dashbrr.local" {
		t.Fatalf("Email = %q, want %q", user.Email, "someuser@dashbrr.local")
	}
}

// A single arg is below cobra.RangeArgs(2, 3)'s minimum and must exit non-zero
// before ever touching args[1]/args[2].
func TestUserCreateArgs_OneArgFails(t *testing.T) {
	t.Chdir(t.TempDir())

	cmd := UserCreateCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"someuser"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a non-zero exit for a single arg")
	}
}
