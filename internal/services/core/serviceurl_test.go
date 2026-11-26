// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"errors"
	"testing"
)

func TestValidateServiceURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "http", raw: "http://autobrr:7474"},
		{name: "https with base path", raw: "https://host/autobrr"},
		{name: "highest port", raw: "http://autobrr:65535"},
		{name: "userinfo", raw: "http://admin:secret@nzbget:6789"}, //nolint:gosec // test fixture
		{name: "empty", raw: "", wantErr: "invalid service URL: must start with http:// or https://"},
		{name: "userinfo without scheme", raw: "admin:secret@nzbget:6789", wantErr: "invalid service URL: must start with http:// or https://"}, //nolint:gosec // test fixture
		{name: "host and port without scheme", raw: "nzbget:6789", wantErr: "invalid service URL: must start with http:// or https://"},
		{name: "ftp", raw: "ftp://host", wantErr: "invalid service URL: must start with http:// or https://"},
		{name: "no host", raw: "http://", wantErr: "invalid service URL: has no host"},
		{name: "port without host name", raw: "http://:8080", wantErr: "invalid service URL: has no host"},
		{name: "port above 65535", raw: "http://autobrr:74747", wantErr: "invalid service URL: port must be at most 65535"},
		{name: "unparseable", raw: "http://admin:secret@nzbget:port", wantErr: "invalid service URL: cannot be parsed"}, //nolint:gosec // test fixture
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateServiceURL(tt.raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateServiceURL(%q) = %v, want nil", tt.raw, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateServiceURL(%q) = %v, want %q", tt.raw, err, tt.wantErr)
			}
			if !errors.Is(err, ErrInvalidServiceURL) {
				t.Fatalf("ValidateServiceURL(%q) = %v, want it to wrap ErrInvalidServiceURL", tt.raw, err)
			}
		})
	}
}
