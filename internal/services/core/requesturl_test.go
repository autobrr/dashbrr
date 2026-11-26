// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestRequestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		elems []string
		want  string
	}{
		{name: "plain", base: "http://sonarr:8989", elems: []string{"api/v3/health"}, want: "http://sonarr:8989/api/v3/health"},
		{name: "leading slash on path", base: "http://sonarr:8989", elems: []string{"/api/v3/health"}, want: "http://sonarr:8989/api/v3/health"},
		{name: "trailing slash on base", base: "http://maintainerr:6246/", elems: []string{"/api/app/status"}, want: "http://maintainerr:6246/api/app/status"},
		{name: "sub-path", base: "https://example.test/sonarr", elems: []string{"/api/v3/health"}, want: "https://example.test/sonarr/api/v3/health"},
		{name: "sub-path with trailing slash", base: "https://example.test/sonarr/", elems: []string{"/api/v3/health"}, want: "https://example.test/sonarr/api/v3/health"},
		{name: "double slashes", base: "https://example.test//sonarr///", elems: []string{"//api//v3/", "/health"}, want: "https://example.test/sonarr/api/v3/health"},
		{name: "surrounding spaces", base: "  http://sonarr:8989/  ", elems: []string{"api"}, want: "http://sonarr:8989/api"},
		{name: "several pieces", base: "http://sonarr:8989", elems: []string{"/api/v3/queue", "42"}, want: "http://sonarr:8989/api/v3/queue/42"},
		{name: "base query kept", base: "http://sabnzbd:8080/?token=abc", elems: []string{"/api"}, want: "http://sabnzbd:8080/api?token=abc"},
		{name: "piece escaped", base: "http://sonarr:8989", elems: []string{"/api/v3/queue", "a b?c#d"}, want: "http://sonarr:8989/api/v3/queue/a%20b%3Fc%23d"},
		{name: "escaped slash stays escaped", base: "http://sonarr:8989", elems: []string{"/api/v3/queue", url.PathEscape("a/b")}, want: "http://sonarr:8989/api/v3/queue/a%2Fb"},
		{name: "escaped base path kept", base: "http://host/my%2Fapp", elems: []string{"/api"}, want: "http://host/my%2Fapp/api"},
		{name: "escaped base path with trailing slash kept", base: "http://host/my%2Fapp/", elems: []string{"/api"}, want: "http://host/my%2Fapp/api"},
		{name: "no pieces", base: "http://plex:32400/", want: "http://plex:32400/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := RequestURL(tt.base, tt.elems...)
			if err != nil {
				t.Fatalf("RequestURL(%q, %q) error: %v", tt.base, tt.elems, err)
			}
			if got != tt.want {
				t.Fatalf("RequestURL(%q, %q) = %q, want %q", tt.base, tt.elems, got, tt.want)
			}
		})
	}
}

func TestRequestURLRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		elems []string
	}{
		{name: "empty base", base: "  "},
		{name: "no scheme", base: "sonarr:8989"},
		{name: "no host", base: "http://"},
		{name: "relative base", base: "/sonarr"},
		{name: "unparseable base", base: "http://host:port"},
		{name: "parent piece", base: "http://sonarr:8989/sonarr", elems: []string{"/api", ".."}},
		{name: "parent inside a path", base: "http://sonarr:8989/sonarr", elems: []string{"/api/../../x"}},
		{name: "dot piece", base: "http://sonarr:8989", elems: []string{"."}},
		{name: "broken escape", base: "http://sonarr:8989", elems: []string{"/api", "100%"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := RequestURL(tt.base, tt.elems...); err == nil {
				t.Fatalf("RequestURL(%q, %q) = %q, want an error", tt.base, tt.elems, got)
			}
		})
	}
}

func TestRequestURLErrorOmitsSecrets(t *testing.T) {
	t.Parallel()

	_, err := RequestURL("http://user:hunter2@host:port/?token=secret", "/api")
	if err == nil {
		t.Fatal("RequestURL accepted an unparseable URL")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error holds a secret: %v", err)
	}
}

func TestRequestURLQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		query url.Values
		elems []string
		want  string
	}{
		{name: "adds query", base: "http://overseerr:5055/", query: url.Values{"take": {"10"}}, elems: []string{"/api/v1/request"}, want: "http://overseerr:5055/api/v1/request?take=10"},
		{name: "merges with base query", base: "http://sabnzbd:8080/?token=abc", query: url.Values{"mode": {"version"}}, elems: []string{"/api"}, want: "http://sabnzbd:8080/api?mode=version&token=abc"},
		{name: "escapes values", base: "http://host", query: url.Values{"q": {"a b&c=d"}}, elems: []string{"/api"}, want: "http://host/api?q=a+b%26c%3Dd"},
		{name: "nil query keeps base query as written", base: "http://host/?b=2&a=1", elems: []string{"/api"}, want: "http://host/api?b=2&a=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := RequestURLQuery(tt.base, tt.query, tt.elems...)
			if err != nil {
				t.Fatalf("RequestURLQuery error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("RequestURLQuery(%q, %v, %q) = %q, want %q", tt.base, tt.query, tt.elems, got, tt.want)
			}
		})
	}
}
