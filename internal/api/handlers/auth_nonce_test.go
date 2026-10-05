package handlers

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestExtractJWTNonce(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"nonce":"abc"}`))
		token := strings.Join([]string{header, payload, "sig"}, ".")

		got, err := extractJWTNonce(token)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "abc" {
			t.Fatalf("expected nonce abc, got %q", got)
		}
	})

	t.Run("invalid_format", func(t *testing.T) {
		_, err := extractJWTNonce("not-a-jwt")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("bad_base64", func(t *testing.T) {
		_, err := extractJWTNonce("a.b@@.c")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("missing_nonce", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
		token := strings.Join([]string{header, payload, "sig"}, ".")

		_, err := extractJWTNonce(token)
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
