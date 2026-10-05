package discovery

import (
	"os"
	"testing"
)

func TestResolveEnvVar(t *testing.T) {
	t.Setenv("DASHBRR_TEST_KEY", "abc123")

	got, err := resolveEnvVar("${DASHBRR_TEST_KEY}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("got %q, want %q", got, "abc123")
	}
}

func TestResolveEnvVar_Missing(t *testing.T) {
	_ = os.Unsetenv("DASHBRR_TEST_MISSING")

	_, err := resolveEnvVar("${DASHBRR_TEST_MISSING}")
	if err == nil {
		t.Fatalf("expected error")
	}
}
